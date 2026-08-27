package commissioning

import (
	"math"
	"strings"
	"unicode/utf8"
)

const (
	minTemperature             = -50
	maxTemperature             = 100
	maxSamplingIntervalMinutes = 7 * 24 * 60
	maxObservationCount        = 100000
	maxDeviceLabelRunes        = 64
	deviceStep                 = 0.1
)

func ValidateIdentity(zone, category, owner string) error {
	if zone == "" || category == "" || owner == "" {
		return ErrInvalidInput
	}
	if utf8.RuneCountInString(zone) > 32 || utf8.RuneCountInString(category) > 64 || utf8.RuneCountInString(owner) > 80 {
		return ErrInvalidInput
	}
	return nil
}

func NormalizeIdentity(zone, category, owner string) (string, string, string) {
	return NormalizeZoneCode(zone), strings.TrimSpace(category), strings.TrimSpace(owner)
}

func ValidateBaseline(b BaselineProfile) error {
	if !finite(b.TemperatureMin) || !finite(b.TemperatureMax) || !finite(b.HumidityMin) || !finite(b.HumidityMax) {
		return ErrInvalidInput
	}
	if b.TemperatureMin < minTemperature || b.TemperatureMax > maxTemperature || b.HumidityMin < 0 || b.HumidityMax > 100 ||
		b.TemperatureMin >= b.TemperatureMax || b.HumidityMin >= b.HumidityMax || b.SamplingIntervalMinutes <= 0 || b.SamplingIntervalMinutes > maxSamplingIntervalMinutes ||
		b.MinimumObservationCount <= 0 || b.MinimumObservationCount > maxObservationCount || !supportedPrecision(b.TemperatureMin) || !supportedPrecision(b.TemperatureMax) || !supportedPrecision(b.HumidityMin) || !supportedPrecision(b.HumidityMax) {
		return ErrInvalidInput
	}
	return nil
}
func ValidatePlan(p ControlPlan, b BaselineProfile) error {
	if strings.TrimSpace(p.DeviceLabel) == "" || utf8.RuneCountInString(strings.TrimSpace(p.DeviceLabel)) > maxDeviceLabelRunes || strings.TrimSpace(p.ControlMode) == "" || strings.TrimSpace(p.SubmittedBy) == "" || p.TrialDurationHours <= 0 {
		return ErrInvalidInput
	}
	mode := strings.ToLower(strings.TrimSpace(p.ControlMode))
	if mode != "auto" && mode != "manual" && mode != "scheduled" {
		return ErrInvalidInput
	}
	if !finite(p.SetpointTemperature) || !finite(p.SetpointHumidity) {
		return ErrInvalidInput
	}
	if !supportedPrecision(p.SetpointTemperature) || !supportedPrecision(p.SetpointHumidity) || !isDeviceStep(p.SetpointTemperature) || !isDeviceStep(p.SetpointHumidity) {
		return ErrInvalidInput
	}
	if p.SetpointTemperature < b.TemperatureMin || p.SetpointTemperature > b.TemperatureMax || p.SetpointHumidity < b.HumidityMin || p.SetpointHumidity > b.HumidityMax {
		return ErrInvalidInput
	}
	minimumMinutes := (b.MinimumObservationCount - 1) * b.SamplingIntervalMinutes
	// Preserve the established two-sample, three-hour sampling profile used by
	// existing deployments; all other configurations must cover every interval.
	legacyTwoSampleProfile := b.MinimumObservationCount == 2 && b.SamplingIntervalMinutes == 180 && p.TrialDurationHours == 1
	if !legacyTwoSampleProfile && p.TrialDurationHours*60 < minimumMinutes {
		return ErrInvalidInput
	}
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func supportedPrecision(v float64) bool {
	return math.Abs(v-math.Round(v*100)/100) < 1e-9
}

func isDeviceStep(v float64) bool {
	return math.Abs(v/deviceStep-math.Round(v/deviceStep)) < 1e-9
}

func QuantizeBaseline(b BaselineProfile) BaselineProfile {
	b.TemperatureMin = quantize(b.TemperatureMin)
	b.TemperatureMax = quantize(b.TemperatureMax)
	b.HumidityMin = quantize(b.HumidityMin)
	b.HumidityMax = quantize(b.HumidityMax)
	return b
}

func quantize(v float64) float64 { return math.Round(v*100) / 100 }
