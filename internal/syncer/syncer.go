package syncer

import (
	"fmt"
	"regexp"

	"github.com/containers/image/v5/signature"

	"github.com/wolfbolin/sync-docker/internal/config"
	"github.com/wolfbolin/sync-docker/internal/logger"
	"github.com/wolfbolin/sync-docker/internal/registry"
)

type Syncer struct {
	dockerHub *registry.DockerHubClient
	harbor    *registry.HarborClient
	syncCfg   *config.SyncConfig
	cfg       *config.Config
	policyCtx *signature.PolicyContext
}

func NewSyncer(
	dockerHub *registry.DockerHubClient,
	harbor *registry.HarborClient,
	syncCfg *config.SyncConfig,
	cfg *config.Config,
) *Syncer {
	policyCtx, err := newPolicyContext()
	if err != nil {
		logger.Fatal("Failed to create policy context: %v", err)
	}

	return &Syncer{
		dockerHub: dockerHub,
		harbor:    harbor,
		syncCfg:   syncCfg,
		cfg:       cfg,
		policyCtx: policyCtx,
	}
}

func (s *Syncer) SetHarborClient(harbor *registry.HarborClient) {
	s.harbor = harbor
}

func (s *Syncer) Close() {
	if s.policyCtx != nil {
		s.policyCtx.Destroy()
	}
}

func (s *Syncer) SyncRule(rule config.Rule) SyncStats {
	var stats SyncStats

	destPr := config.ParseRef(rule.Dest)
	destTags, err := s.harbor.ListTagsWithDigest(destPr.Project, destPr.Name)
	if err != nil {
		logger.Error("Failed to fetch target tags for %s/%s: %v", destPr.Project, destPr.Name, err)
		return stats
	}

	srcTags := s.fetchSourceTags(rule)
	tagsToSync := s.resolveTags(rule, srcTags, destTags)

	for _, tag := range tagsToSync {
		srcRef := config.BuildRef(config.ParseRef(rule.Source), tag.Name)
		dstRef := config.BuildRef(destPr, tag.Name)
		logger.Info("  Sync: %s => %s", srcRef, dstRef)
		if err := s.copyImage(srcRef, dstRef, rule); err != nil {
			logger.Error("  ✗ Failed: %v", err)
			stats.Failed++
		} else {
			logger.Done("  ✓ Success")
			stats.Success++
		}
	}

	return stats
}

func (s *Syncer) CheckRule(rule config.Rule) (*CheckResult, error) {
	destPr := config.ParseRef(rule.Dest)
	result := &CheckResult{Source: rule.Source, Dest: rule.Dest}

	destTags, err := s.harbor.ListTagsWithDigest(destPr.Project, destPr.Name)
	if err != nil {
		return nil, fmt.Errorf("fetch target tags: %w", err)
	}

	destDigestMap := make(map[string]string, len(destTags))
	for _, t := range destTags {
		destDigestMap[t.Name] = t.Digest
	}

	srcTags := s.fetchSourceTags(rule)

	if len(rule.Tags) > 0 {
		result.TagMode = "tags"
		result.Matched = rule.Tags
		srcDigestMap := buildSrcDigestMap(srcTags)
		for _, tag := range rule.Tags {
			classifyTag(tag, srcDigestMap, destDigestMap, result)
		}
	}

	if rule.TagRegex != "" {
		result.TagMode = "tag_regex"
		result.TagRegex = rule.TagRegex
		result.TotalTags = len(srcTags)
		re := compilePattern(rule.TagRegex)
		srcDigestMap := buildSrcDigestMap(srcTags)
		for _, tag := range srcTags {
			if re != nil && !re.MatchString(tag.Name) {
				continue
			}
			result.Matched = append(result.Matched, tag.Name)
			if isV1Manifest(tag.MediaType) {
				result.SkippedV1 = append(result.SkippedV1, tag.Name)
				continue
			}
			classifyTag(tag.Name, srcDigestMap, destDigestMap, result)
		}
	}

	return result, nil
}

func (s *Syncer) fetchSourceTags(rule config.Rule) []registry.DockerHubTag {
	srcPr := config.ParseRef(rule.Source)
	tags, err := s.dockerHub.ListTags(srcPr.Project, srcPr.Name)
	if err != nil {
		logger.Error("Failed to fetch source tags for %s/%s: %v", srcPr.Project, srcPr.Name, err)
		return nil
	}
	return tags
}

type tagAction struct {
	Name   string
	Sync   bool
	Reason string
}

func (s *Syncer) resolveTags(rule config.Rule, srcTags []registry.DockerHubTag, destTags []registry.HarborTagInfo) []tagAction {
	destDigestMap := make(map[string]string, len(destTags))
	for _, t := range destTags {
		destDigestMap[t.Name] = t.Digest
	}

	srcDigestMap := buildSrcDigestMap(srcTags)

	if len(rule.Tags) > 0 {
		var actions []tagAction
		for _, tag := range rule.Tags {
			destDigest, exists := destDigestMap[tag]
			if !exists {
				actions = append(actions, tagAction{Name: tag, Sync: true})
			} else if srcDigest, ok := srcDigestMap[tag]; ok && srcDigest != "" && srcDigest != destDigest {
				logger.Warn("  Update: %s (digest changed)", tag)
				actions = append(actions, tagAction{Name: tag, Sync: true})
			} else {
				logger.Warn("  Skip: %s (up-to-date)", tag)
			}
		}
		return actions
	}

	if rule.TagRegex != "" {
		re := compilePattern(rule.TagRegex)
		var actions []tagAction
		for _, tag := range srcTags {
			if re != nil && !re.MatchString(tag.Name) {
				continue
			}
			if isV1Manifest(tag.MediaType) {
				logger.Warn("  Skip: %s (v1 manifest)", tag.Name)
				continue
			}
			destDigest, exists := destDigestMap[tag.Name]
			if !exists {
				actions = append(actions, tagAction{Name: tag.Name, Sync: true})
			} else if tag.Digest != "" && tag.Digest != destDigest {
				logger.Warn("  Update: %s (digest changed)", tag.Name)
				actions = append(actions, tagAction{Name: tag.Name, Sync: true})
			} else {
				logger.Warn("  Skip: %s (up-to-date)", tag.Name)
			}
		}
		return actions
	}

	return nil
}

func buildSrcDigestMap(tags []registry.DockerHubTag) map[string]string {
	m := make(map[string]string, len(tags))
	for _, t := range tags {
		m[t.Name] = t.Digest
	}
	return m
}

func classifyTag(name string, srcDigestMap, destDigestMap map[string]string, result *CheckResult) {
	destDigest, exists := destDigestMap[name]
	if !exists {
		result.ToSync = append(result.ToSync, name)
	} else if srcDigest, ok := srcDigestMap[name]; ok && srcDigest != "" && srcDigest != destDigest {
		result.Updated = append(result.Updated, name)
	} else {
		result.Exist = append(result.Exist, name)
	}
}

func compilePattern(pattern string) *regexp.Regexp {
	re, _ := regexp.Compile(pattern)
	return re
}

func isV1Manifest(mediaType string) bool {
	return mediaType == "application/vnd.docker.distribution.manifest.v1+json" ||
		mediaType == "application/vnd.docker.distribution.manifest.v1+prettyjws"
}
