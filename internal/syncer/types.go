package syncer

type SyncStats struct {
	Success int
	Failed  int
	Exist   int
	Skipped int
}

func (s *SyncStats) Add(other SyncStats) {
	s.Success += other.Success
	s.Failed += other.Failed
	s.Exist += other.Exist
	s.Skipped += other.Skipped
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
	SkippedV1 []string
	ToSync    []string
}
