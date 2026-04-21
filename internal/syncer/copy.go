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

func isSchema1MediaType(mediaType string) bool {
	return mediaType == "application/vnd.docker.distribution.manifest.v1+json" ||
		mediaType == "application/vnd.docker.distribution.manifest.v1+prettyjws"
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

	preserveDigests := true
	if idx := strings.LastIndex(srcRef, ":"); idx > 0 {
		repository := srcRef[:idx]
		tag := srcRef[idx+1:]
		mediaType, err := s.sourceClient.GetManifestMediaType(repository, tag)
		if err != nil {
			logger.Warn("  Cannot check manifest type for %s: %v", srcRef, err)
		} else if isSchema1MediaType(mediaType) {
			logger.Warn("  Schema1 detected for %s, disabling PreserveDigests", srcRef)
			preserveDigests = false
		}
	}

	options := &copy.Options{
		SourceCtx:          sourceCtx,
		DestinationCtx:     destCtx,
		ImageListSelection: copy.CopyAllImages,
		PreserveDigests:    preserveDigests,
	}

	_, err = copy.Image(ctx, s.policyCtx, dstRefParsed, srcRefParsed, options)
	if err != nil {
		return fmt.Errorf("copy image: %w", err)
	}

	return nil
}
