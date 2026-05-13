package domain

import (
	"errors"
	"time"
)

type BatchStage struct {
	ID          int
	BatchID     int
	StageNumber int
	StageKey    string
	StageName   string
	StartedAt   time.Time
	CompletedAt *time.Time
	SignedBy    *int
	SignedAt    *time.Time
	Comment     string
}

type Event struct {
	ID          int
	BatchID     int
	StageKey    string
	Type        string
	Severity    string
	Description string
	Comment     string
	ResolvedBy  *int
	OccurredAt  time.Time
}

// StageSignCondition defines a sensor condition that must be met before signing a stage.
type StageSignCondition struct {
	SensorCode string
	MinValue   *float64
	MaxValue   *float64
	Unit       string
	Label      string
}

// ProcessStage describes a stage in the emulsification process.
type ProcessStage struct {
	Number       int
	Key          string
	Name         string
	Instruction  string // single-line summary (backward compat)
	Instructions []string
	StageSensors []string
	Conditions   []StageSignCondition
}

// ConditionStatus carries the real-time state of a single sign condition.
type ConditionStatus struct {
	SensorCode string  `json:"sensor_code"`
	Label      string  `json:"label"`
	Current    float64 `json:"current"`
	Unit       string  `json:"unit"`
	Met        bool    `json:"met"`
	HasReading bool    `json:"has_reading"`
}

func fp(v float64) *float64 { return &v }

// AllStages defines the canonical 18-stage emulsification sequence.
var AllStages = []ProcessStage{
	{
		Number: 1, Key: "water_pot_feeding", Name: "Загрузка водной фазы",
		Instruction: "Загрузите компоненты водной фазы в водный котёл.",
		Instructions: []string{
			"Отмерьте воду очищенную (65,6 %), глицерин (3,0 %) и ТЭА (0,5 %) согласно журналу взвешивания",
			"Загрузите все компоненты водной фазы в водный котёл",
			"Проверьте герметичность котла и подключение датчиков WP-TEMP-01, WP-WEIGHT-01",
			"Убедитесь, что клапан подачи закрыт",
			"Нажмите «Подписать стадию» для подтверждения загрузки",
		},
		StageSensors: []string{"WP-WEIGHT-01", "WP-TEMP-01"},
		Conditions:   []StageSignCondition{},
	},
	{
		Number: 2, Key: "oil_pot_feeding", Name: "Загрузка масляной фазы",
		Instruction: "Загрузите компоненты масляной фазы в масляный котёл.",
		Instructions: []string{
			"Отмерьте масляную фазу: масло виноградных косточек (5,0 %), кокосовое масло (5,0 %), МГД (5,0 %), воск (4,0 %), ланолин (3,0 %), кремофор А25 (2,0 %)",
			"Загрузите все компоненты в масляный котёл",
			"Убедитесь в подключении датчиков OP-TEMP-02, OP-WEIGHT-02",
			"Нажмите «Подписать стадию» для подтверждения загрузки",
		},
		StageSensors: []string{"OP-WEIGHT-02", "OP-TEMP-02"},
		Conditions:   []StageSignCondition{},
	},
	{
		Number: 3, Key: "main_pot_vacuumize", Name: "Вакуумирование основного котла",
		Instruction: "Запустите вакуумный насос, добейтесь MP-VACUUM-01 ≤ −0,05 МПа.",
		Instructions: []string{
			"Проверьте герметичность крышки и фланцев основного котла",
			"Закройте все клапаны подачи",
			"Запустите вакуумный насос",
			"Следите за MP-VACUUM-01 — вакуум должен достигнуть ≤ −0,05 МПа",
			"После стабилизации вакуума нажмите «Подписать стадию»",
		},
		StageSensors: []string{"MP-VACUUM-01", "MP-TEMP-03"},
		Conditions: []StageSignCondition{
			{SensorCode: "MP-VACUUM-01", MaxValue: fp(-0.05), Unit: "MPa", Label: "MP-VACUUM-01 ≤ −0,05 МПа"},
		},
	},
	{
		Number: 4, Key: "water_pot_heating", Name: "Нагрев водной фазы",
		Instruction: "Нагрев водного котла до 80 °C (мешалка 200 об/мин).",
		Instructions: []string{
			"Включите нагрев водного котла",
			"Установите скорость мешалки WP-MIXER-01 на 200 об/мин",
			"Контролируйте WP-TEMP-01 — целевое 80 °C, допустимый диапазон 75–85 °C",
			"При T > 85 °C снизьте нагрев — возможна деградация ТЭА",
			"Кнопка «Подписать» активируется при WP-TEMP-01 ≥ 75 °C",
		},
		StageSensors: []string{"WP-TEMP-01", "WP-MIXER-01"},
		Conditions: []StageSignCondition{
			{SensorCode: "WP-TEMP-01", MinValue: fp(75), Unit: "C", Label: "WP-TEMP-01 ≥ 75 °C"},
		},
	},
	{
		Number: 5, Key: "oil_pot_heating", Name: "Нагрев масляной фазы",
		Instruction: "Нагрев масляного котла до 80 °C — воск должен полностью расплавиться.",
		Instructions: []string{
			"Включите нагрев масляного котла",
			"Установите мешалку OP-MIXER-02 на 200 об/мин",
			"Контролируйте OP-TEMP-02 — целевое 80 °C (диапазон 75–85 °C)",
			"Дождитесь полного расплавления эмульсионного воска (~75–80 °C)",
			"Удерживайте WP-TEMP-01 в диапазоне 75–85 °C",
			"Кнопка «Подписать» активируется при OP-TEMP-02 ≥ 75 °C",
		},
		StageSensors: []string{"OP-TEMP-02", "OP-MIXER-02", "WP-TEMP-01"},
		Conditions: []StageSignCondition{
			{SensorCode: "OP-TEMP-02", MinValue: fp(75), Unit: "C", Label: "OP-TEMP-02 ≥ 75 °C"},
		},
	},
	{
		Number: 6, Key: "main_pot_preheating", Name: "Предварительный нагрев основного котла",
		Instruction: "Прогрейте основной котёл до 75 °C.",
		Instructions: []string{
			"Включите нагрев основного котла",
			"Контролируйте MP-TEMP-03 — целевое значение 75 °C",
			"Убедитесь, что водный и масляный котлы удерживают 80 °C",
			"Кнопка «Подписать» активируется при MP-TEMP-03 ≥ 70 °C",
		},
		StageSensors: []string{"MP-TEMP-03", "WP-TEMP-01", "OP-TEMP-02"},
		Conditions: []StageSignCondition{
			{SensorCode: "MP-TEMP-03", MinValue: fp(70), Unit: "C", Label: "MP-TEMP-03 ≥ 70 °C"},
		},
	},
	{
		Number: 7, Key: "main_pot_water_feeding", Name: "Подача водной фазы в основной котёл",
		Instruction: "Передайте водную фазу из водного котла в основной котёл.",
		Instructions: []string{
			"Убедитесь, что WP-TEMP-01 ≥ 75 °C (водная фаза готова)",
			"Откройте клапан подачи из водного котла в основной",
			"Контролируйте прирост массы MP-WEIGHT-03 и убыль WP-WEIGHT-01",
			"Поддерживайте перемешивание в основном котле",
			"После завершения подачи закройте клапан и нажмите «Подписать стадию»",
		},
		StageSensors: []string{"WP-WEIGHT-01", "MP-WEIGHT-03", "WP-TEMP-01", "MP-TEMP-03"},
		Conditions:   []StageSignCondition{},
	},
	{
		Number: 8, Key: "main_pot_pre_blending", Name: "Предварительное смешение (200 об/мин)",
		Instruction: "Мешалка 200 об/мин, продолжительность 10 мин.",
		Instructions: []string{
			"Включите гомогенизатор основного котла на 200 об/мин",
			"Поддерживайте MP-TEMP-03 ≈ 80 °C",
			"Продолжайте перемешивание 10 мин (тест: 5 мин)",
			"Контролируйте равномерность",
			"Кнопка «Подписать» активируется при MP-HOMOG-01 ≥ 100 об/мин",
		},
		StageSensors: []string{"MP-HOMOG-01", "MP-TEMP-03"},
		Conditions: []StageSignCondition{
			{SensorCode: "MP-HOMOG-01", MinValue: fp(100), Unit: "rpm", Label: "MP-HOMOG-01 ≥ 100 об/мин"},
		},
	},
	{
		Number: 9, Key: "main_pot_vacuum_drawing_1", Name: "Вакуумирование 1",
		Instruction: "Вакуум в основном котле, MP-VACUUM-01 ≤ −0,05 МПа.",
		Instructions: []string{
			"Включите вакуумный насос основного котла",
			"Контролируйте MP-VACUUM-01 — целевое ≤ −0,05 МПа",
			"Поддерживайте перемешивание и температуру",
			"Нажмите «Подписать стадию» после достижения вакуума",
		},
		StageSensors: []string{"MP-VACUUM-01", "MP-TEMP-03", "MP-HOMOG-01"},
		Conditions: []StageSignCondition{
			{SensorCode: "MP-VACUUM-01", MaxValue: fp(-0.05), Unit: "MPa", Label: "MP-VACUUM-01 ≤ −0,05 МПа"},
		},
	},
	{
		Number: 10, Key: "main_pot_oil_feeding", Name: "Подача масляной фазы (подтверждение T)",
		Instruction: "OP-TEMP-02 ≥ 75 °C, постепенно подайте масляную фазу.",
		Instructions: []string{
			"Убедитесь, что OP-TEMP-02 ≥ 75 °C (масляная фаза готова)",
			"Убедитесь, что MP-TEMP-03 ≥ 70 °C",
			"Откройте клапан подачи масляной фазы",
			"Подавайте постепенно при непрерывном перемешивании",
			"Контролируйте OP-WEIGHT-02 (убыль) и MP-WEIGHT-03 (прирост)",
			"После подачи закройте клапан и нажмите «Подписать стадию»",
		},
		StageSensors: []string{"OP-TEMP-02", "MP-TEMP-03", "OP-WEIGHT-02", "MP-WEIGHT-03"},
		Conditions: []StageSignCondition{
			{SensorCode: "OP-TEMP-02", MinValue: fp(75), Unit: "C", Label: "OP-TEMP-02 ≥ 75 °C"},
		},
	},
	{
		Number: 11, Key: "main_pot_vacuum_drawing_2", Name: "Вакуумирование 2",
		Instruction: "Повторное вакуумирование, MP-VACUUM-01 ≤ −0,05 МПа.",
		Instructions: []string{
			"Включите вакуум повторно",
			"Контролируйте MP-VACUUM-01 ≤ −0,05 МПа",
			"Убедитесь в удалении воздушных включений из эмульсии",
			"Нажмите «Подписать стадию» после достижения вакуума",
		},
		StageSensors: []string{"MP-VACUUM-01", "MP-TEMP-03"},
		Conditions: []StageSignCondition{
			{SensorCode: "MP-VACUUM-01", MaxValue: fp(-0.05), Unit: "MPa", Label: "MP-VACUUM-01 ≤ −0,05 МПа"},
		},
	},
	{
		Number: 12, Key: "emulsifying_speed_2", Name: "Эмульгирование — скорость 2 (2000 об/мин)",
		Instruction: "Гомогенизатор 2000 об/мин, продолжительность 10 мин.",
		Instructions: []string{
			"Плавно увеличьте MP-HOMOG-01 до 2000 об/мин",
			"Контролируйте MP-TEMP-03 — допустимый диапазон 75–85 °C",
			"При T > 85 °C снизьте скорость и зафиксируйте отклонение",
			"Продолжайте гомогенизацию 10 мин при 2000 об/мин",
			"Кнопка «Подписать» активируется при MP-HOMOG-01 ≥ 1800 об/мин",
		},
		StageSensors: []string{"MP-HOMOG-01", "MP-TEMP-03", "MP-VACUUM-01"},
		Conditions: []StageSignCondition{
			{SensorCode: "MP-HOMOG-01", MinValue: fp(1800), Unit: "rpm", Label: "MP-HOMOG-01 ≥ 1800 об/мин"},
		},
	},
	{
		Number: 13, Key: "emulsifying_speed_3", Name: "Эмульгирование — скорость 3 (2000 об/мин)",
		Instruction: "Гомогенизация 2000 об/мин ещё 20 мин. Итого: 30 мин.",
		Instructions: []string{
			"Продолжайте гомогенизацию при 2000 об/мин",
			"Длительность — 20 мин (суммарно 30 мин эмульгирования)",
			"Следите за MP-TEMP-03 — держать 75–85 °C",
			"Визуально контролируйте формирование однородной эмульсии",
			"Кнопка «Подписать» активируется при MP-HOMOG-01 ≥ 1800 об/мин",
		},
		StageSensors: []string{"MP-HOMOG-01", "MP-TEMP-03"},
		Conditions: []StageSignCondition{
			{SensorCode: "MP-HOMOG-01", MinValue: fp(1800), Unit: "rpm", Label: "MP-HOMOG-01 ≥ 1800 об/мин"},
		},
	},
	{
		Number: 14, Key: "cooling_start", Name: "Начало охлаждения",
		Instruction: "Запустите охлаждение, снизьте скорость до 200 об/мин.",
		Instructions: []string{
			"Отключите нагрев и включите подачу хладагента в рубашку котла",
			"Плавно снизьте скорость гомогенизатора до 200 об/мин",
			"Зафиксируйте время начала охлаждения",
			"Нажмите «Подписать стадию» для подтверждения запуска охлаждения",
		},
		StageSensors: []string{"MP-TEMP-03", "MP-HOMOG-01"},
		Conditions:   []StageSignCondition{},
	},
	{
		Number: 15, Key: "cooling_blending", Name: "Охлаждение при перемешивании (200 об/мин)",
		Instruction: "Охлаждение до MP-TEMP-03 ≤ 40 °C при 200 об/мин.",
		Instructions: []string{
			"Поддерживайте скорость перемешивания 200 об/мин",
			"Контролируйте MP-TEMP-03 — целевое ≤ 40 °C",
			"Не ускоряйте охлаждение резко — нарушение структуры эмульсии",
			"Кнопка «Подписать» активируется при MP-TEMP-03 ≤ 40 °C",
		},
		StageSensors: []string{"MP-TEMP-03", "MP-HOMOG-01"},
		Conditions: []StageSignCondition{
			{SensorCode: "MP-TEMP-03", MaxValue: fp(40), Unit: "C", Label: "MP-TEMP-03 ≤ 40 °C"},
		},
	},
	{
		Number: 16, Key: "additive_feeding", Name: "Внесение добавок (T 30–35 °C)",
		Instruction: "T ≤ 35 °C — вносите добавки.",
		Instructions: []string{
			"КРИТИЧНО: убедитесь, что MP-TEMP-03 ≤ 35 °C (добавки термолабильны!)",
			"Добавьте экстракт бадана толстолистного и салициловую кислоту",
			"Добавьте диметикон и ментол при перемешивании",
			"Добавьте октопирокс, эуксил 9010 и эфирное масло розового дерева",
			"Перемешивайте 2–3 мин после каждого добавления",
			"Кнопка «Подписать» активируется при MP-TEMP-03 ≤ 35 °C",
		},
		StageSensors: []string{"MP-TEMP-03"},
		Conditions: []StageSignCondition{
			{SensorCode: "MP-TEMP-03", MaxValue: fp(35), Unit: "C", Label: "MP-TEMP-03 ≤ 35 °C"},
		},
	},
	{
		Number: 17, Key: "final_blending", Name: "Финальное перемешивание (15 мин)",
		Instruction: "Перемешивайте 200 об/мин ещё 15 мин.",
		Instructions: []string{
			"Установите гомогенизатор на 200 об/мин",
			"Перемешивайте 15 мин до однородной консистенции",
			"Визуально контролируйте отсутствие комков и расслоений",
			"Кнопка «Подписать» активируется при MP-HOMOG-01 ≥ 100 об/мин",
		},
		StageSensors: []string{"MP-HOMOG-01", "MP-TEMP-03"},
		Conditions: []StageSignCondition{
			{SensorCode: "MP-HOMOG-01", MinValue: fp(100), Unit: "rpm", Label: "MP-HOMOG-01 ≥ 100 об/мин"},
		},
	},
	{
		Number: 18, Key: "cooling_finish", Name: "Завершение охлаждения и контроль pH",
		Instruction: "Охладите до комнатной T, pH 5,0–9,0, завершите партию.",
		Instructions: []string{
			"Охладите крем до комнатной температуры (MP-TEMP-03 ≤ 30 °C)",
			"Измерьте pH готового продукта — норма: 5,0–9,0",
			"Оцените визуально: консистенция, однородность, цвет, запах",
			"Отберите контрольный образец для лабораторного анализа",
			"При соответствии всем параметрам нажмите «Подписать стадию» для завершения партии",
		},
		StageSensors: []string{"MP-TEMP-03"},
		Conditions: []StageSignCondition{
			{SensorCode: "MP-TEMP-03", MaxValue: fp(30), Unit: "C", Label: "MP-TEMP-03 ≤ 30 °C"},
		},
	},
}

// StageByKey finds a ProcessStage by its key.
func StageByKey(key string) (ProcessStage, bool) {
	for _, s := range AllStages {
		if s.Key == key {
			return s, true
		}
	}
	return ProcessStage{}, false
}

var (
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrEquipmentOffline   = errors.New("equipment is offline or not ready")
	ErrStageNotFound      = errors.New("stage not found")
	ErrStageAlreadySigned = errors.New("stage already signed")
	ErrEventNotFound      = errors.New("event not found")
	ErrBatchCompleted     = errors.New("batch completed")
	ErrBatchCancelled     = errors.New("batch cancelled")
	ErrNotProcessOperator = errors.New("only the operator who started the process can sign stages")
	ErrConditionNotMet    = errors.New("stage conditions not met")
)
