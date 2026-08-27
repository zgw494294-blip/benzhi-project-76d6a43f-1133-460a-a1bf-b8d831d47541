package commissioning

import "errors"

var (
	ErrInvalidInput        = errors.New("参数校验失败")
	ErrInvalidTransition   = errors.New("当前状态不允许此操作")
	ErrVersionConflict     = errors.New("档案版本冲突")
	ErrNotFound            = errors.New("档案不存在")
	ErrIndependentReviewer = errors.New("复核员必须独立于方案提交人")
	ErrOpenDeviation       = errors.New("仍有未关闭偏差")
	ErrObservationOrder    = errors.New("观测序号必须连续且不可覆盖")
	ErrPermitNotFound      = errors.New("启用许可不存在")
	ErrInvalidObservation  = errors.New("观测数据无效")
	ErrPackageStale        = errors.New("复核资料包已过期")
	ErrRemediationTarget   = errors.New("整改目标偏差无效")
	ErrPermitStorage       = errors.New("启用许可读取失败")
	ErrStorageCorrupt      = errors.New("档案存储一致性错误")
	ErrIdempotencyConflict = errors.New("幂等键对应的请求冲突")
	ErrReviewHistory       = errors.New("复核历史一致性错误")
	ErrExportInvalid       = errors.New("档案导出失败")
)
