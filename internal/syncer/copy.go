package syncer

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"

	"github.com/wolfbolin/sync-docker/internal/config"
	"github.com/wolfbolin/sync-docker/internal/logger"
)

var errSchema1 = fmt.Errorf("schema1 image skipped")

func isSchema1Error(err error) bool {
	return strings.Contains(err.Error(), "schema1") ||
		strings.Contains(err.Error(), "would change the manifest")
}

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

		if isSchema1Error(lastErr) {
			return errSchema1
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

	logger.Debug("  copy.Image: docker://%s => docker://%s (proxy=%v)", srcRef, dstRef, rule.Proxy)

	sourceCtx := &types.SystemContext{}
	destCtx := &types.SystemContext{}

	if rule.Proxy && s.proxyURL != "" {
		proxyURL, err := url.Parse(s.proxyURL)
		if err != nil {
			return fmt.Errorf("parse proxy URL %q: %w", s.proxyURL, err)
		}
		sourceCtx.DockerProxyURL = proxyURL
	}

	options := &copy.Options{
		SourceCtx:          sourceCtx,
		DestinationCtx:     destCtx,
		ImageListSelection: copy.CopyAllImages,
		PreserveDigests:    true,
	}

	_, err = copy.Image(ctx, s.policyCtx, dstRefParsed, srcRefParsed, options)
	if err != nil {
		return fmt.Errorf("copy image: %w", err)
	}

	return nil
}
