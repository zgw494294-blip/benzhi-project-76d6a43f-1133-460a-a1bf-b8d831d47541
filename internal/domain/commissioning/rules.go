package commissioning

import "time"

type RuleResult struct {
	Code        string
	Severity    string
	Description string
}

func EvaluateObservation(o TrialObservation, b BaselineProfile) []RuleResult {
	r := []RuleResult{}
	if o.Temperature < b.TemperatureMin || o.Temperature > b.TemperatureMax {
		r = append(r, RuleResult{"TEMP_RANGE", "high", "温度超出锁定基线区间"})
	}
	if o.Humidity < b.HumidityMin || o.Humidity > b.HumidityMax {
		r = append(r, RuleResult{"HUMIDITY_RANGE", "high", "湿度超出锁定基线区间"})
	}
	if o.DeviceStatus == DeviceAbnormal {
		r = append(r, RuleResult{"DEVICE_STATUS", "high", "调控设备状态异常"})
	}
	return r
}
func TrialWindowSatisfied(obs []TrialObservation, p ControlPlan, b BaselineProfile) bool {
	if len(obs) < b.MinimumObservationCount {
		return false
	}
	return obs[len(obs)-1].ObservedAt.Sub(obs[0].ObservedAt) >= time.Duration(p.TrialDurationHours)*time.Hour
}
