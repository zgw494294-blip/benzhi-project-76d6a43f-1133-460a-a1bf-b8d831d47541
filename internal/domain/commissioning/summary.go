package commissioning

import "time"

func (c *CommissioningCase) SummarizeObservations(from, to *time.Time) (ObservationSummary, error) {
	if from != nil && to != nil && from.After(*to) {
		return ObservationSummary{}, ErrInvalidInput
	}
	result := ObservationSummary{From: from, To: to, OpenDeviationCounts: map[string]int{}, ResolvedDeviationCounts: map[string]int{}}
	effective := c.EffectiveObservations()
	filtered := make([]TrialObservation, 0, len(effective))
	for _, observation := range effective {
		if from != nil && observation.ObservedAt.Before(*from) {
			continue
		}
		if to != nil && observation.ObservedAt.After(*to) {
			continue
		}
		filtered = append(filtered, observation)
	}
	result.EffectiveObservationCount = len(filtered)
	if len(filtered) > 0 {
		tempMin, tempMax := filtered[0].Temperature, filtered[0].Temperature
		humMin, humMax := filtered[0].Humidity, filtered[0].Humidity
		var tempSum, humSum float64
		for _, observation := range filtered {
			if observation.Temperature < tempMin {
				tempMin = observation.Temperature
			}
			if observation.Temperature > tempMax {
				tempMax = observation.Temperature
			}
			if observation.Humidity < humMin {
				humMin = observation.Humidity
			}
			if observation.Humidity > humMax {
				humMax = observation.Humidity
			}
			tempSum += observation.Temperature
			humSum += observation.Humidity
		}
		result.TemperatureMin, result.TemperatureMax = &tempMin, &tempMax
		result.HumidityMin, result.HumidityMax = &humMin, &humMax
		result.TemperatureAverage = tempSum / float64(len(filtered))
		result.HumidityAverage = humSum / float64(len(filtered))
		result.DurationMinutes = int64(filtered[len(filtered)-1].ObservedAt.Sub(filtered[0].ObservedAt) / time.Minute)
	}
	for _, deviation := range c.Deviations {
		if deviation.Status == DeviationOpen {
			result.OpenDeviationCounts[deviation.RuleCode]++
		}
		if deviation.Status == DeviationResolved {
			result.ResolvedDeviationCounts[deviation.RuleCode]++
		}
	}
	if c.Baseline != nil {
		result.ObservationRequired = c.Baseline.MinimumObservationCount
		result.DurationRequiredMinutes = 0
		if c.Plan != nil {
			result.DurationRequiredMinutes = c.Plan.TrialDurationHours * 60
		}
		result.ObservationProgress = result.EffectiveObservationCount
		if result.ObservationRequired > 0 {
			result.ObservationCompletion = clampProgress(float64(result.ObservationProgress) / float64(result.ObservationRequired))
		}
		if result.DurationRequiredMinutes > 0 {
			result.DurationCompletion = clampProgress(float64(result.DurationMinutes) / float64(result.DurationRequiredMinutes))
		}
	}
	return result, nil
}

func clampProgress(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
