package _interface

//go:generate go tool mockgen -source=i_background_job.go -destination=mocks/mock_i_background_job.go -package=mocks

// IBackgroundJob is work the system does on its own, with nobody asking. Starting
// it must not block: a job that needs to keep running does so on its own.
type IBackgroundJob interface {
	Start()
}
