package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func Test(t *testing.T) {
	if os.Getenv("KTUNNELS_E2E_TEST") != "1" {
		t.Skipf("skip because KTUNNELS_E2E_TEST=1 is not set")
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	var wg wait.Group
	wg.StartWithContext(ctx, func(ctx context.Context) {
		defer cancel()
		if err := runKubectl(ctx, "port-forward", "svc/payment-db", "10002:80"); err != nil {
			t.Errorf("kubectl port-foward error: %s", err)
		}
	})
	wg.StartWithContext(ctx, func(ctx context.Context) {
		defer cancel()
		const endpoint = "http://localhost:10002/get"
		if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
			if err := httpGet(t, ctx, endpoint); err != nil {
				t.Logf("retrying: %s", err)
				return false, nil
			}
			return true, nil
		}); err != nil {
			t.Errorf("endpoint %s did not become success: %s", endpoint, err)
		}
	})
	wg.Wait()
}

func runKubectl(ctx context.Context, args ...string) error {
	c := exec.Command("kubectl", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	<-ctx.Done()
	if err := c.Process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("send SIGINT: %w", err)
	}
	if err := c.Wait(); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	return nil
}

func httpGet(t *testing.T, ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("could not create a request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("could not close response body: %s", err)
		}
	}()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status code wants 200 but was %d", resp.StatusCode)
	}
	return nil
}
