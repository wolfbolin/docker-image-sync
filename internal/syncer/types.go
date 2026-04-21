package syncer

import "github.com/wolfbolin/sync-docker/internal/config"

type SyncStats struct {
	Success int
	Failed  int
	Exist   int
}

func (s *SyncStats) Add(other SyncStats) {
	s.Success += other.Success
	s.Failed += other.Failed
	s.Exist += other.Exist
}

type DeleteStats struct {
	Deleted int
	Failed  int
	Skipped int
}

func (s *DeleteStats) Add(other DeleteStats) {
	s.Deleted += other.Deleted
	s.Failed += other.Failed
	s.Skipped += other.Skipped
}

type DeleteItem struct {
	TagName string
	Digest  string
	Reason  string
}

type DeleteResult struct {
	Dest      string
	TagMode   string
	TagRegex  string
	Tags      []string
	TotalTags int
	Unmatched []DeleteItem
	Kept      []string
}

type CheckResult struct {
	Source    string
	Dest      string
	TagMode   string
	TagRegex  string
	TotalTags int
	Matched   []string
	Exist     []string
	Updated   []string
	ToSync    []string
}

type SyncResult struct {
	Source    string
	Dest      string
	TagMode   string
	TagRegex  string
	TotalTags int
	ToSync    []string
	Updated   []string
	Exist     []string
	Stats     SyncStats

	tagsToSync []tagAction
	destPr     config.ParsedRef
	rule       config.Rule
}
