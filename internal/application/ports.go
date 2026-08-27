package application

import "github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
import "errors"

var ErrMissingIdempotency = errors.New("幂等键不存在")

type Repository interface {
	Save(*commissioning.CommissioningCase, string) error
	Get(string) (*commissioning.CommissioningCase, error)
	Cases() ([]*commissioning.CommissioningCase, error)
	FindPermit(string) (*commissioning.ActivationPermit, error)
	GetIdempotency(string) (*IdempotentResult, error)
	SaveIdempotency(string, IdempotentResult) error
}

type SnapshotExporter interface {
	ExportSnapshot(string, string) (ExportResult, error)
}

// RequestFingerprintSaver lets repositories commit a mutation and its
// idempotency fingerprint under the same repository lock.
type RequestFingerprintSaver interface {
	SaveWithFingerprint(*commissioning.CommissioningCase, string, string) error
}

type ExportResult struct {
	ByteCount        int    `json:"byteCount"`
	RelativeFileName string `json:"relativeFileName"`
	SHA256           string `json:"sha256"`
}
type IdempotentResult struct {
	Status      int    `json:"status"`
	Body        []byte `json:"body"`
	Fingerprint string `json:"fingerprint,omitempty"`
}
