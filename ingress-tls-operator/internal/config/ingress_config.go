package config

import (
	"context"
	"encoding/base64"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type IngressWatcherOperatorConfig struct {
	HTTPTimeout          time.Duration
	TrustAnchors         []byte
	InsecureSkipVerify   bool
	PeriodicScanInterval time.Duration
	PeriodicScanWorkers  int
}

func LoadWithDefault(ctx context.Context, c client.Client, configFile string,
	defaultConf *IngressWatcherOperatorConfig) *IngressWatcherOperatorConfig {
	conf, err := Load(ctx, c, configFile)
	if err != nil {
		return defaultConf
	}
	// Set mandatory defaults
	if conf.HTTPTimeout == 0 {
		conf.HTTPTimeout = 60 * time.Second
	}
	if conf.PeriodicScanInterval == 0 {
		conf.PeriodicScanInterval = 60 * time.Minute
	}
	if conf.PeriodicScanWorkers <= 0 {
		conf.PeriodicScanWorkers = 5
	}
	return conf
}

func Load(ctx context.Context, c client.Client, configFile string) (*IngressWatcherOperatorConfig, error) {
	logger := log.FromContext(ctx)

	ns := "default"
	if namespace := os.Getenv("POD_NAMESPACE"); len(namespace) > 0 {
		ns = namespace
	}

	key := client.ObjectKey{
		Namespace: ns,
		Name:      configFile,
	}

	cfg := &IngressWatcherOperatorConfig{
		HTTPTimeout: 30 * time.Second,
	}

	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, key, cm); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("config configmap not found, using defaults")
			return cfg, nil
		}
		return nil, err
	}

	if val, ok := cm.Data["http-timeout"]; ok {
		if duration, err := time.ParseDuration(val); err == nil {
			cfg.HTTPTimeout = duration
		} else {
			logger.Error(err, "invalid http-timeout, using default")
			cfg.HTTPTimeout = 30 * time.Second
		}
	}

	if val, ok := cm.Data["tls-insecure"]; ok {
		if tlsInsecure, err := strconv.ParseBool(val); err == nil {
			cfg.InsecureSkipVerify = tlsInsecure
		} else {
			logger.Error(err, "invalid tls-insecure, using default")
			cfg.InsecureSkipVerify = false
		}
	}

	if val, ok := cm.Data["tls-trustanchors"]; ok {
		if rawData, err := base64.StdEncoding.DecodeString(val); err == nil {
			cfg.TrustAnchors = rawData
		} else {
			logger.Error(err, "invalid tls-insecure, using default")
			cfg.TrustAnchors = make([]byte, 0)
		}
	}

	if val, ok := cm.Data["periodic-scan-interval"]; ok {
		if duration, err := time.ParseDuration(val); err == nil {
			cfg.PeriodicScanInterval = duration
		} else {
			logger.Error(err, "invalid periodic-scan-interval, using default")
			cfg.PeriodicScanInterval = 60 * time.Minute
		}
	}

	if val, ok := cm.Data["periodic-scan-workers"]; ok {
		if workersCount, err := strconv.Atoi(val); err == nil {
			cfg.PeriodicScanWorkers = workersCount
		} else {
			logger.Error(err, "invalid periodic-scan-interval, using default")
			cfg.PeriodicScanWorkers = 5
		}
	}

	return cfg, nil
}
