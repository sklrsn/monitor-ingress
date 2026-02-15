package scan

import (
	"context"
	"sync"
	"time"

	"github.com/sklrsn/monitor-ingress/internal/validator"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PeriodicScanner struct {
	client               client.Client
	periodicScanInterval time.Duration
	periodicScanWorkers  int
	validator            *validator.IngressValidator
}

func (ps *PeriodicScanner) work(ctx context.Context, dataCh chan *networkingv1.Ingress) {
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-dataCh:
			if !ok {
				return
			}
			scanResponse := ps.validator.Inspect(ctx, data)
			scanResponse.Log(ctx)
		}
	}
}
func (ps *PeriodicScanner) Scan(ctx context.Context) {
	logger := log.FromContext(ctx)
	ticker := time.NewTicker(ps.periodicScanInterval)
	defer ticker.Stop()

	dataCh := make(chan *networkingv1.Ingress, ps.periodicScanWorkers)

	var wg sync.WaitGroup
	for i := 1; i <= ps.periodicScanWorkers; i++ {
		wg.Go(func() {
			ps.work(ctx, dataCh)
		})
	}

	go func() {
		<-ctx.Done()
		logger.Info("context cancelled, closing data channel")
		close(dataCh)
	}()

	for {
		select {
		case <-ctx.Done():
			logger.Info("waiting for workers to finish")
			wg.Wait()
			logger.Info("periodic scanner stopped")
			return
		case <-ticker.C:
			ingressList := &networkingv1.IngressList{}
			if err := ps.client.List(ctx, ingressList); err != nil {
				logger.Error(err, "list ingress")
			}
			for _, ingress := range ingressList.Items {
				select {
				case <-ctx.Done():
					return
				case dataCh <- &ingress:
				}
			}
		}
	}

}

func WithScanInterval(interval time.Duration) func(p *PeriodicScanner) {
	return func(p *PeriodicScanner) {
		p.periodicScanInterval = interval
	}
}

func WithScanWorkers(workers int) func(p *PeriodicScanner) {
	return func(p *PeriodicScanner) {
		p.periodicScanWorkers = workers
	}
}

func NewPeriodicScanner(ctx context.Context, ingressValidator *validator.IngressValidator,
	c client.Client, args ...func(*PeriodicScanner)) *PeriodicScanner {
	scr := &PeriodicScanner{
		validator:            ingressValidator,
		client:               c,
		periodicScanInterval: 60 * time.Minute,
		periodicScanWorkers:  5,
	}
	for _, f := range args {
		f(scr)
	}
	return scr
}
