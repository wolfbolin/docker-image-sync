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
	sourceClient registry.SourceClient
	harbor       *registry.HarborClient
	syncCfg      *config.SyncConfig
	cfg          *config.Config
	policyCtx    *signature.PolicyContext
	proxyURL     string
}

func NewSyncer(
	sourceClient registry.SourceClient,
	harbor *registry.HarborClient,
	syncCfg *config.SyncConfig,
	cfg *config.Config,
) *Syncer {
	policyCtx, err := newPolicyContext()
	if err != nil {
		logger.Fatal("Failed to create policy context: %v", err)
	}

	return &Syncer{
		sourceClient: sourceClient,
		harbor:       harbor,
		syncCfg:      syncCfg,
		cfg:          cfg,
		policyCtx:    policyCtx,
		proxyURL:     cfg.GetProxy(),
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

func (s *Syncer) applyProxy(rule config.Rule) {
	if rule.Proxy && s.proxyURL != "" {
		s.sourceClient.SetProxy(s.proxyURL)
	} else {
		s.sourceClient.SetProxy("")
	}
}

func (s *Syncer) SyncRuleDetailed(rule config.Rule) (*SyncResult, error) {
	result := &SyncResult{Source: rule.Source, Dest: rule.Dest}

	destPr := config.ParseRef(rule.Dest)
	destTags, err := s.harbor.ListTagsWithDigest(destPr.Project, destPr.Name)
	if err != nil {
		return nil, fmt.Errorf("fetch target tags for %s/%s: %w", destPr.Project, destPr.Name, err)
	}

	srcTags := s.fetchSourceTags(rule)
	tagsToSync, existTags, err := s.resolveTags(rule, srcTags, destTags)
	if err != nil {
		return nil, err
	}

	filteredActions, schema1Tags := s.checkSchema1AndFilter(rule, tagsToSync)

	result.Schema1 = schema1Tags
	result.Exist = existTags
	result.Stats.Schema1 = len(schema1Tags)

	if len(rule.Tags) > 0 {
		result.TagMode = "tags"
	} else if rule.TagRegex != "" {
		result.TagMode = "tag_regex"
		result.TagRegex = rule.TagRegex
		result.TotalTags = len(srcTags)
	}

	for _, tag := range filteredActions {
		if tag.Reason == "new" {
			result.ToSync = append(result.ToSync, tag.Name)
		} else if tag.Reason == "update" {
			result.Updated = append(result.Updated, tag.Name)
		}
	}

	for _, tag := range filteredActions {
		srcRef := config.BuildRef(config.ParseRef(rule.Source), tag.Name)
		dstRef := config.BuildRef(destPr, tag.Name)
		logger.Info("  Sync: %s => %s", srcRef, dstRef)
		if err := s.copyImage(srcRef, dstRef, rule); err != nil {
			logger.Error("  ✗ Failed: %v", err)
			result.Stats.Failed++
		} else {
			logger.Done("  ✓ Success")
			result.Stats.Success++
		}
	}

	return result, nil
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
			classifyTag(tag.Name, srcDigestMap, destDigestMap, result)
		}
	}

	return result, nil
}

func (s *Syncer) AnalyzeDeleteRule(rule config.Rule) (*DeleteResult, error) {
	destPr := config.ParseRef(rule.Dest)
	result := &DeleteResult{Dest: rule.Dest}

	artifacts, err := s.harbor.ListArtifacts(destPr.Project, destPr.Name)
	if err != nil {
		return nil, fmt.Errorf("fetch artifacts for %s/%s: %w", destPr.Project, destPr.Name, err)
	}

	result.TotalTags = 0
	for _, a := range artifacts {
		result.TotalTags += len(a.Tags)
	}

	if len(rule.Tags) > 0 {
		result.TagMode = "tags"
		result.Tags = rule.Tags
		tagSet := make(map[string]bool, len(rule.Tags))
		for _, t := range rule.Tags {
			tagSet[t] = true
		}
		for _, artifact := range artifacts {
			if isSchema1MediaType(artifact.MediaType) {
				for _, t := range artifact.Tags {
					result.Schema1 = append(result.Schema1, DeleteItem{
						TagName: t.Name, Digest: artifact.Digest, Reason: "schema1",
					})
				}
				continue
			}
			for _, t := range artifact.Tags {
				if tagSet[t.Name] {
					result.Kept = append(result.Kept, t.Name)
				} else {
					result.Unmatched = append(result.Unmatched, DeleteItem{
						TagName: t.Name, Digest: artifact.Digest, Reason: "not in tags list",
					})
				}
			}
		}
		return result, nil
	}

	if rule.TagRegex != "" {
		result.TagMode = "tag_regex"
		result.TagRegex = rule.TagRegex
		re := compilePattern(rule.TagRegex)
		for _, artifact := range artifacts {
			if isSchema1MediaType(artifact.MediaType) {
				for _, t := range artifact.Tags {
					result.Schema1 = append(result.Schema1, DeleteItem{
						TagName: t.Name, Digest: artifact.Digest, Reason: "schema1",
					})
				}
				continue
			}
			for _, t := range artifact.Tags {
				if re != nil && re.MatchString(t.Name) {
					result.Kept = append(result.Kept, t.Name)
				} else {
					result.Unmatched = append(result.Unmatched, DeleteItem{
						TagName: t.Name, Digest: artifact.Digest, Reason: "not matching regex",
					})
				}
			}
		}
		return result, nil
	}

	for _, artifact := range artifacts {
		if isSchema1MediaType(artifact.MediaType) {
			for _, t := range artifact.Tags {
				result.Schema1 = append(result.Schema1, DeleteItem{
					TagName: t.Name, Digest: artifact.Digest, Reason: "schema1",
				})
			}
		} else {
			for _, t := range artifact.Tags {
				result.Kept = append(result.Kept, t.Name)
			}
		}
	}

	return result, nil
}

func (s *Syncer) DeleteRule(rule config.Rule, dryRun bool) DeleteStats {
	result, err := s.AnalyzeDeleteRule(rule)
	if err != nil {
		logger.Error("%v", err)
		return DeleteStats{}
	}

	destPr := config.ParseRef(rule.Dest)
	var stats DeleteStats

	toDelete := append(result.Schema1, result.Unmatched...)

	if dryRun {
		stats.Skipped = len(toDelete)
		return stats
	}

	deletedDigests := make(map[string]bool)
	for _, item := range toDelete {
		if deletedDigests[item.Digest] {
			continue
		}
		logger.Info("  Delete: %s (%s)", item.TagName, item.Reason)
		if err := s.harbor.DeleteArtifact(destPr.Project, destPr.Name, item.Digest); err != nil {
			logger.Error("  ✗ Failed: %v", err)
			stats.Failed++
		} else {
			logger.Done("  ✓ Deleted")
			stats.Deleted++
			deletedDigests[item.Digest] = true
		}
	}

	return stats
}

func isSchema1MediaType(mediaType string) bool {
	return mediaType == "application/vnd.docker.distribution.manifest.v1+json" ||
		mediaType == "application/vnd.docker.distribution.manifest.v1+prettyjws"
}

func (s *Syncer) fetchSourceTags(rule config.Rule) []registry.SourceTag {
	s.applyProxy(rule)

	repository := rule.Source

	tags, err := s.sourceClient.ListTags(repository)
	if err != nil {
		logger.Error("Failed to fetch source tags for %s: %v", repository, err)
		return nil
	}
	return tags
}

type tagAction struct {
	Name   string
	Sync   bool
	Reason string
}

func (s *Syncer) resolveTags(rule config.Rule, srcTags []registry.SourceTag, destTags []registry.HarborTagInfo) ([]tagAction, []string, error) {
	destDigestMap := make(map[string]string, len(destTags))
	for _, t := range destTags {
		destDigestMap[t.Name] = t.Digest
	}

	srcDigestMap := buildSrcDigestMap(srcTags)

	if len(rule.Tags) > 0 {
		var actions []tagAction
		var existTags []string
		for _, tag := range rule.Tags {
			destDigest, exists := destDigestMap[tag]
			if !exists {
				actions = append(actions, tagAction{Name: tag, Sync: true, Reason: "new"})
			} else if srcDigest, ok := srcDigestMap[tag]; ok && srcDigest != "" && srcDigest != destDigest {
				logger.Warn("  Update: %s (digest changed)", tag)
				actions = append(actions, tagAction{Name: tag, Sync: true, Reason: "update"})
			} else {
				logger.Warn("  Skip: %s (up-to-date)", tag)
				existTags = append(existTags, tag)
			}
		}
		return actions, existTags, nil
	}

	if rule.TagRegex != "" {
		re := compilePattern(rule.TagRegex)
		var actions []tagAction
		var existTags []string
		for _, tag := range srcTags {
			if re != nil && !re.MatchString(tag.Name) {
				continue
			}
			destDigest, exists := destDigestMap[tag.Name]
			if !exists {
				actions = append(actions, tagAction{Name: tag.Name, Sync: true, Reason: "new"})
			} else if tag.Digest != "" && tag.Digest != destDigest {
				logger.Warn("  Update: %s (digest changed)", tag.Name)
				actions = append(actions, tagAction{Name: tag.Name, Sync: true, Reason: "update"})
			} else {
				logger.Warn("  Skip: %s (up-to-date)", tag.Name)
				existTags = append(existTags, tag.Name)
			}
		}
		return actions, existTags, nil
	}

	return nil, nil, fmt.Errorf("rule has neither tags nor tag_regex specified")
}

func (s *Syncer) checkSchema1AndFilter(rule config.Rule, actions []tagAction) ([]tagAction, []string) {
	s.applyProxy(rule)

	var filtered []tagAction
	var schema1Tags []string

	for _, action := range actions {
		mediaType, err := s.sourceClient.GetManifestMediaType(rule.Source, action.Name)
		if err != nil {
			logger.Warn("  Cannot check manifest type for %s: %v", action.Name, err)
			filtered = append(filtered, action)
			continue
		}

		if isSchema1MediaType(mediaType) {
			logger.Warn("  Skip: %s (schema1 at source)", action.Name)
			schema1Tags = append(schema1Tags, action.Name)
			continue
		}

		filtered = append(filtered, action)
	}

	return filtered, schema1Tags
}

func buildSrcDigestMap(tags []registry.SourceTag) map[string]string {
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
