package code_executor

import "time"

const (
	ContainerMemoryLimit       = "256m"
	ContainerCPULimit          = "0.5"
	ContainerHealthCheckTimeout = 100 * time.Millisecond
	ContainerPoolCheckInterval = 30 * time.Second
)

const (
	CircuitBreakerFailureThreshold = 5
	CircuitBreakerCooldownPeriod   = 30 * time.Second
)

const (
	ExecutionDirPrefix = "/tmp/exec_"
	TestCaseDelimiter  = "---END_TEST_CASE---"
	OutputStartMarker  = "---OUTPUT_START---"
	OutputEndMarker    = "---OUTPUT_END---"
)

const (
	RedisExecutionQueue  = "execution_queue"
	RedisJobStatusPrefix = "job_status:"
	RedisJobResultPrefix = "job_result:"
)

const (
	RedisProcessorDivisor = 5
	MinRedisProcessors    = 100
	BatchSize             = 10
	PollInterval          = 100 * time.Millisecond
	MetricsReportInterval = 30 * time.Second
)