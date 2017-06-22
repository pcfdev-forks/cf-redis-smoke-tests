package service_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/cf-test-helpers/services"

	"github.com/pcfdev-forks/cf-redis-smoke-tests/retry"
	"github.com/pcfdev-forks/cf-redis-smoke-tests/service/reporter"
)

type retryConfig struct {
	BaselineMilliseconds uint   `json:"baseline_interval_milliseconds"`
	Attempts             uint   `json:"max_attempts"`
	BackoffAlgorithm     string `json:"backoff"`
}

func (rc retryConfig) Backoff() retry.Backoff {
	baseline := time.Duration(rc.BaselineMilliseconds) * time.Millisecond

	algo := strings.ToLower(rc.BackoffAlgorithm)

	switch algo {
	case "linear":
		return retry.Linear(baseline)
	case "exponential":
		return retry.Linear(baseline)
	default:
		return retry.None(baseline)
	}
}

func (rc retryConfig) MaxRetries() int {
	return int(rc.Attempts)
}

type redisTestConfig struct {
	ServiceName string      `json:"service_name"`
	PlanNames   []string    `json:"plan_names"`
	Retry       retryConfig `json:"retry"`
}

func loadCFTestConfig(path string) services.Config {
	config := services.Config{}

	if err := services.LoadConfig(path, &config); err != nil {
		panic(err)
	}

	if err := services.ValidateConfig(&config); err != nil {
		panic(err)
	}

	config.TimeoutScale = 3

	return config
}

func loadRedisTestConfig(path string) redisTestConfig {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	config := redisTestConfig{}
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		panic(err)
	}

	return config
}

var (
	configPath   = os.Getenv("CONFIG_PATH")
	cfTestConfig = loadCFTestConfig(configPath)
	redisConfig  = loadRedisTestConfig(configPath)

	smokeTestReporter *reporter.SmokeTestReport
)

func TestService(t *testing.T) {
	smokeTestReporter = new(reporter.SmokeTestReport)

	reporter := []Reporter{
		Reporter(smokeTestReporter),
	}

	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "P-Redis Smoke Tests", reporter)
}
