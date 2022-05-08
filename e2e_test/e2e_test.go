package e2e_test

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/wait"
	"log"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
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
		if err := wait.PollUntilWithContext(ctx, 3*time.Second, getContent); err != nil {
			t.Errorf("get content error: %s", err)
		}
	})
	wg.StartWithContext(ctx, func(ctx context.Context) {
		defer cancel()
		if err := runPortForward(ctx); err != nil {
			t.Errorf("kubectl port-foward error: %s", err)
		}
	})
	wg.Wait()
}

func runPortForward(ctx context.Context) error {
	c := exec.CommandContext(ctx, "kubectl", "port-forward", "svc/payment-db", "10000:80")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("kubectl port-forward: %w", err)
	}
	return nil
}

func getContent(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:10000/get", nil)
	if err != nil {
		return false, fmt.Errorf("could not create a request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("received a response %s", resp.Status)
	return true, nil
}
