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

// ProcessStage describes a stage in the emulsification process.
type ProcessStage struct {
	Number      int
	Key         string
	Name        string
	Instruction string
}

// AllStages defines the canonical 18-stage emulsification sequence.
var AllStages = []ProcessStage{
	{1, "water_pot_feeding", "Загрузка водной фазы", "Загрузите водную фазу (вода очищенная, глицерин, ТЭА) в водный котёл. Убедитесь в правильности состава и массы контейнеров."},
	{2, "oil_pot_feeding", "Загрузка масляной фазы", "Загрузите масляную фазу (масло виноградных косточек, кокосовое масло, МГД, воск, ланолин, кремофор А25) в масляный котёл."},
	{3, "main_pot_vacuumize", "Вакуумирование основного котла", "Запустите вакуумный насос. Контроль: MP-VACUUM-01 должен достигнуть < −0.05 МПа."},
	{4, "water_pot_heating", "Нагрев водной фазы", "Включите нагрев и мешалку водного котла (200 об/мин). Целевая температура: 80 °C. Допустимый диапазон: 75–85 °C."},
	{5, "oil_pot_heating", "Нагрев масляной фазы", "Включите нагрев масляного котла. Целевая температура: 80 °C. Воск должен полностью расплавиться. Допустимый диапазон: 75–85 °C."},
	{6, "main_pot_preheating", "Предварительный нагрев основного котла", "Прогрейте основной котёл до 75 °C перед подачей водной фазы."},
	{7, "main_pot_water_feeding", "Подача водной фазы в основной котёл", "Передайте водную фазу из водного котла в основной котёл. Контролируйте массу по MP-WEIGHT-03."},
	{8, "main_pot_pre_blending", "Предварительное смешение (200 об/мин)", "Включите мешалку основного котла на 200 об/мин. Продолжительность: 10 мин (тест: 5 мин)."},
	{9, "main_pot_vacuum_drawing_1", "Вакуумирование 1", "Включите вакуум в основном котле. Контроль: MP-VACUUM-01 < −0.05 МПа."},
	{10, "main_pot_oil_feeding", "Подача масляной фазы (подтверждение T)", "Убедитесь, что T масляной фазы = 80 °C (OP-TEMP-02). Подайте масляную фазу постепенно при постоянном перемешивании."},
	{11, "main_pot_vacuum_drawing_2", "Вакуумирование 2", "Повторное вакуумирование после подачи масляной фазы. Контроль: MP-VACUUM-01 < −0.05 МПа."},
	{12, "emulsifying_speed_2", "Эмульгирование — скорость 2 (2000 об/мин)", "Увеличьте скорость гомогенизатора до 2000 об/мин. Продолжительность: 10 мин. Контроль T: 80 °C."},
	{13, "emulsifying_speed_3", "Эмульгирование — скорость 3 (2000 об/мин)", "Продолжайте гомогенизацию 2000 об/мин ещё 20 мин. Итого гомогенизация: 30 мин."},
	{14, "cooling_start", "Начало охлаждения", "Запустите охлаждение. Начните снижение скорости до 200 об/мин."},
	{15, "cooling_blending", "Охлаждение при перемешивании (200 об/мин)", "Продолжайте перемешивание 200 об/мин до достижения T ≤ 40 °C. Контроль: MP-TEMP-03."},
	{16, "additive_feeding", "Внесение добавок (подтверждение T)", "Убедитесь, что T = 30–35 °C. Внесите добавки: экстракт бадана, салициловую кислоту, диметикон, ментол, октопирокс, эуксил 9010, эфирное масло. Перемешайте."},
	{17, "final_blending", "Финальное перемешивание (15 мин)", "Перемешивайте 200 об/мин ещё 15 мин до однородной консистенции."},
	{18, "cooling_finish", "Завершение охлаждения и контроль pH", "Охладите до комнатной температуры. Измерьте pH (норма: 5.0–9.0). Подтвердите партию."},
}

var (
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrEquipmentOffline   = errors.New("equipment is offline or not ready")
	ErrStageNotFound      = errors.New("stage not found")
	ErrStageAlreadySigned = errors.New("stage already signed")
	ErrEventNotFound      = errors.New("event not found")
	ErrBatchCompleted          = errors.New("batch completed")
	ErrNotProcessOperator      = errors.New("only the operator who started the process can sign stages")
)
