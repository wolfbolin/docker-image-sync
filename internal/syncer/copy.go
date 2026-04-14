package syncer

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"

	"github.com/wolfbolin/sync-docker/internal/config"
	"github.com/wolfbolin/sync-docker/internal/logger"
)

func newPolicyContext() (*signature.PolicyContext, error) {
	policy := &signature.Policy{
		Default: []signature.PolicyRequirement{
			signature.NewPRInsecureAcceptAnything(),
		},
	}
	return signature.NewPolicyContext(policy)
}

func (s *Syncer) copyImage(srcRef, dstRef string, rule config.Rule) error {
	var lastErr error
	retryCount := s.syncCfg.RetryCount()

	for attempt := 1; attempt <= retryCount; attempt++ {
		if attempt > 1 {
			interval := s.syncCfg.RetryDuration()
			logger.Warn("  Retry %d/%d after %v", attempt, retryCount, interval)
			time.Sleep(interval)
		}

		lastErr = s.doCopy(srcRef, dstRef, rule)
		if lastErr == nil {
			return nil
		}

		logger.Error("  Attempt %d failed: %v", attempt, lastErr)
	}

	return fmt.Errorf("all %d attempts failed, last error: %w", retryCount, lastErr)
}

func (s *Syncer) doCopy(srcRef, dstRef string, rule config.Rule) error {
	ctx := context.Background()

	srcRefParsed, err := alltransports.ParseImageName("docker://" + srcRef)
	if err != nil {
		return fmt.Errorf("parse source ref: %w", err)
	}

	dstRefParsed, err := alltransports.ParseImageName("docker://" + dstRef)
	if err != nil {
		return fmt.Errorf("parse destination ref: %w", err)
	}

	if rule.Proxy {
		if proxy := s.cfg.GetProxy(); proxy != "" {
			os.Setenv("HTTPS_PROXY", proxy)
			os.Setenv("HTTP_PROXY", proxy)
		}
		if noProxy := s.cfg.GetNoProxy(); noProxy != "" {
			os.Setenv("NO_PROXY", noProxy)
			os.Setenv("no_proxy", noProxy)
		}
	} else {
		os.Unsetenv("HTTPS_PROXY")
		os.Unsetenv("HTTP_PROXY")
		os.Unsetenv("NO_PROXY")
		os.Unsetenv("no_proxy")
	}

	logger.Debug("  copy.Image: docker://%s => docker://%s (proxy=%v)", srcRef, dstRef, rule.Proxy)

	options := &copy.Options{
		SourceCtx:          &types.SystemContext{},
		DestinationCtx:     &types.SystemContext{},
		ImageListSelection: copy.CopyAllImages,
		PreserveDigests:    true,
	}

	_, err = copy.Image(ctx, s.policyCtx, dstRefParsed, srcRefParsed, options)
	if err != nil {
		return fmt.Errorf("copy image: %w", err)
	}

	return nil
}
