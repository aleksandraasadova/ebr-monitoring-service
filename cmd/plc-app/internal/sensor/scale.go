package sensor

// датчик весов (этап взвешивание)

type Scale struct {
	currentRaw uint16
}

func (s *Scale) SimulateStep(targetGrams uint16) uint16 {
	if s.currentRaw >= targetGrams {
		s.currentRaw = targetGrams
		return targetGrams
	}
	halfTarget := targetGrams / 2

	if s.currentRaw < halfTarget {
		s.currentRaw = halfTarget
		return s.currentRaw
	}

	step := (targetGrams / 2 / 5)
	s.currentRaw += step
	return s.currentRaw
}

func (s *Scale) Reset() {
	s.currentRaw = 0
}
