package sync

import (
	"context"
	"fmt"
	"regexp"
	"slices"

	"github.com/cockroachdb/errors"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/wolfbolin/bolbox/pkg/log"
	"github.com/wolfbolin/sync-docker/internal/cfg"
	"github.com/wolfbolin/sync-docker/internal/hub"
)

type Syncer struct {
	config       *cfg.Config
	sourceClient hub.Client
	targetClient hub.Client
}

func NewSyncer(config *cfg.Config, sourceClient, targetClient hub.Client) *Syncer {
	return &Syncer{
		config:       config,
		sourceClient: sourceClient,
		targetClient: targetClient,
	}
}

func (s *Syncer) PrepareSyncTags(ctx context.Context, rule *cfg.Rule, cmpDigest bool) (*TagSet, error) {
	sourceImage := hub.ParseImage(rule.Source)
	targetImage := hub.ParseImage(rule.Target)
	log.Infof("Compare image tags from %s & %s", sourceImage.ToUrl(), targetImage.ToUrl())

	sourceTags, err := s.sourceClient.ImageTags(ctx, sourceImage)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	targetTags, err := s.targetClient.ImageTags(ctx, targetImage)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	matchTags, _ := filterTags(sourceTags, rule.Tags, rule.TagRegex)
	tagSet := splitTagSet(matchTags, targetTags)

	if cmpDigest {
		dTag, sTag, err := s.splitByDigest(ctx, sourceImage, targetImage, tagSet.Same)
		if err != nil {
			return nil, err
		}
		tagSet.Diff = dTag
		tagSet.Same = sTag
	}

	return tagSet, nil
}

func (s *Syncer) PrepareDeleteTags(ctx context.Context, rule *cfg.Rule, justLocal bool) (*TagSet, error) {
	sourceImage := hub.ParseImage(rule.Source)
	targetImage := hub.ParseImage(rule.Target)
	log.Infof("Compare image tags from %s", targetImage.ToUrl())

	targetTags, err := s.targetClient.ImageTags(ctx, targetImage)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	tagSet := &TagSet{}
	if !justLocal {
		sourceTags, err := s.sourceClient.ImageTags(ctx, sourceImage)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		matchTags, _ := filterTags(sourceTags, rule.Tags, rule.TagRegex)
		tagSet = splitTagSet(matchTags, targetTags)
	} else {
		tagSet.Same, tagSet.Over = filterTags(targetTags, rule.Tags, rule.TagRegex)
	}

	return tagSet, nil
}

func filterTags(hubTags, ruleTags []string, tagRegex string) ([]string, []string) {
	reg, _ := regexp.Compile(tagRegex)

	var matchTags []string
	var missTags []string
	for _, tag := range hubTags {
		if slices.Contains(ruleTags, tag) {
			matchTags = append(matchTags, tag)
			continue
		}
		if len(tagRegex) != 0 && reg.MatchString(tag) {
			matchTags = append(matchTags, tag)
			continue
		}
		missTags = append(missTags, tag)
	}

	return matchTags, missTags
}

func splitTagSet(sourceTags, targetTags []string) *TagSet {
	tagIndex := make(map[string]int)
	for _, tag := range sourceTags {
		tagIndex[tag] += 1
	}
	for _, tag := range targetTags {
		tagIndex[tag] += 2
	}

	var news []string
	var over []string
	var diff []string
	var same []string
	for key, val := range tagIndex {
		switch val {
		case 1:
			news = append(news, key)
		case 2:
			over = append(over, key)
		case 3:
			same = append(same, key)
		}
	}
	return &TagSet{news, over, same, diff}
}

func (s *Syncer) splitByDigest(
	ctx context.Context,
	sourceImage, targetImage *hub.Image,
	existTags []string,
) ([]string, []string, error) {
	log.Infof("Compare image tag digest for %s & %s", sourceImage.ToUrl(), targetImage.ToUrl())
	var atUpdate []string
	var atExist []string

	for _, tag := range existTags {
		sourceDigest, err := s.sourceClient.ImageTagDigest(ctx, sourceImage, tag)
		if err != nil {
			return nil, nil, err
		}
		log.Debugf("=>Digest %s: %s", sourceImage.ToTagUrl(tag), sourceDigest)
		targetDigest, err := s.targetClient.ImageTagDigest(ctx, targetImage, tag)
		if err != nil {
			return nil, nil, err
		}
		log.Debugf("=>Digest %s: %s", targetImage.ToTagUrl(tag), targetDigest)

		if sourceDigest == targetDigest {
			atExist = append(atExist, tag)
		} else {
			atUpdate = append(atUpdate, tag)
		}
	}
	return atUpdate, atExist, nil
}

func (s *Syncer) ExecuteSync(ctx context.Context, rule *cfg.Rule, tagSet *TagSet) (*RuleSum, error) {
	sourceImage := hub.ParseImage(rule.Source)
	targetImage := hub.ParseImage(rule.Target)

	policy := &signature.Policy{
		Default: []signature.PolicyRequirement{
			signature.NewPRInsecureAcceptAnything(), // 允许所有未签名的镜像
		},
	}
	policyCtx, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to create policy context")
	}
	defer policyCtx.Destroy()

	syncSum := &RuleSum{}
	for _, tag := range tagSet.Sync {
		log.Infof("Copy image: %s => %s", sourceImage.ToTagUrl(tag), targetImage.ToTagUrl(tag))
		isSuccess := s.copyImageWithRetry(ctx, sourceImage, targetImage, tag, policyCtx)
		if isSuccess {
			syncSum.Add = append(syncSum.Add, tag)
		} else {
			syncSum.Err = append(syncSum.Err, tag)
		}
	}
	for _, tag := range tagSet.Diff {
		log.Infof("Diff image: %s => %s", sourceImage.ToTagUrl(tag), targetImage.ToTagUrl(tag))
		isSuccess := s.copyImageWithRetry(ctx, sourceImage, targetImage, tag, policyCtx)
		if isSuccess {
			syncSum.Put = append(syncSum.Put, tag)
		} else {
			syncSum.Err = append(syncSum.Err, tag)
		}
	}

	return syncSum, nil
}

func (s *Syncer) copyImageWithRetry(
	ctx context.Context,
	sourceImage, targetImage *hub.Image,
	tag string,
	policyCtx *signature.PolicyContext,
) bool {
	isSuccess := false
	config := ctx.Value("config").(*cfg.Config)
	for t := range config.Retry.Times {
		if t != 0 {
			log.Warnf("=>Retry copy image(%d/%d)", t, config.Retry.Times-1)
		}

		syncErr := s.copyImage(ctx, sourceImage, targetImage, tag, policyCtx)
		if syncErr != nil {
			log.Errorf("Failed: %s", syncErr)
			continue
		}
		log.Infof("=>Diff image success")
		isSuccess = true
		break
	}
	return isSuccess
}

func (s *Syncer) copyImage(ctx context.Context, sourceImage, targetImage *hub.Image, tag string, policy *signature.PolicyContext) error {
	sourceTagStr := fmt.Sprintf("docker://%s", sourceImage.ToTagUrl(tag))
	sourceTagRef, err := alltransports.ParseImageName(sourceTagStr)
	if err != nil {
		return errors.WithStack(err)
	}
	targetTagStr := fmt.Sprintf("docker://%s", targetImage.ToTagUrl(tag))
	targetTagRef, err := alltransports.ParseImageName(targetTagStr)
	if err != nil {
		return errors.WithStack(err)
	}

	//preserveDigests := true
	//mediaType, err := s.sourceClient.ImageMediaType(ctx, sourceImage, tag)
	//if err != nil {
	//	return errors.WithStack(err)
	//}
	//fmt.Println(mediaType)
	//if hub.IsSchemaV1(mediaType) {
	//	preserveDigests = false
	//}

	copyOpts := &copy.Options{
		SourceCtx:          s.sourceClient.SystemCtx(),
		DestinationCtx:     s.targetClient.SystemCtx(),
		ImageListSelection: copy.CopyAllImages,
		PreserveDigests:    false,
	}

	_, err = copy.Image(ctx, policy, targetTagRef, sourceTagRef, copyOpts)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (s *Syncer) ExecuteDelete(ctx context.Context, rule *cfg.Rule, tagSet *TagSet) (*RuleSum, error) {
	targetImage := hub.ParseImage(rule.Target)

	sum := &RuleSum{}
	for _, tag := range tagSet.Over {
		log.Infof("Delete image: %s", targetImage.ToTagUrl(tag))
		isSuccess := s.deleteImageWithRetry(ctx, targetImage, tag)
		if isSuccess {
			sum.Del = append(sum.Del, tag)
		} else {
			sum.Err = append(sum.Err, tag)
		}
	}

	return sum, nil
}

func (s *Syncer) deleteImageWithRetry(
	ctx context.Context,
	targetImage *hub.Image,
	tag string,
) bool {
	isSuccess := false
	config := ctx.Value("config").(*cfg.Config)
	for t := range config.Retry.Times {
		if t != 0 {
			log.Warnf("=>Retry delete image(%d/%d)", t, config.Retry.Times-1)
		}

		deleteErr := s.deleteImage(ctx, targetImage, tag)
		if deleteErr != nil {
			log.Errorf("Failed: %s", deleteErr)
			continue
		}
		log.Infof("=>Delete image success")
		isSuccess = true
		break
	}
	return isSuccess
}

func (s *Syncer) deleteImage(ctx context.Context, targetImage *hub.Image, tag string) error {
	tagRef, err := alltransports.ParseImageName("docker://" + targetImage.ToTagUrl(tag))
	if err != nil {
		return errors.WithStack(err)
	}
	if err = tagRef.DeleteImage(ctx, s.targetClient.SystemCtx()); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
