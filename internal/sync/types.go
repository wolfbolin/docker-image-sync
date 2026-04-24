package sync

type TagSet struct {
	Sync []string
	Over []string
	Same []string
	Diff []string
}

type RuleSum struct {
	Add []string
	Del []string
	Put []string
	Err []string
}
