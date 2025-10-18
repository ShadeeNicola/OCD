package types

// Automation reporting domain types

// TestReport represents the aggregate Jenkins test report for a build.
type TestReport struct {
	BuildNumber int         `json:"buildNumber"`
	PassCount   int         `json:"passCount"`
	FailCount   int         `json:"failCount"`
	SkipCount   int         `json:"skipCount"`
	Duration    float64     `json:"duration"`
	Suites      []TestSuite `json:"suites"`
}

// TestSuite represents a Jenkins test suite entry.
type TestSuite struct {
	Name     string     `json:"name"`
	Duration float64    `json:"duration"`
	Cases    []TestCase `json:"cases"`
}

// TestCase carries details about a single test execution.
type TestCase struct {
	ClassName       string  `json:"className"`
	Name            string  `json:"name"`
	Status          string  `json:"status"`
	Duration        float64 `json:"duration"`
	ErrorDetails    string  `json:"errorDetails,omitempty"`
	ErrorStackTrace string  `json:"errorStackTrace,omitempty"`
	FailedSince     int     `json:"failedSince,omitempty"`
	Age             int     `json:"age,omitempty"`
}

// AutomationBuildInfo represents high-level build data used by automation analytics.
type AutomationBuildInfo struct {
	Number    int    `json:"number"`
	Result    string `json:"result"`
	Duration  int64  `json:"duration"`
	Timestamp int64  `json:"timestamp"`
}

// BuildComparison summarises the delta between two builds.
type BuildComparison struct {
	BuildA           int               `json:"buildA"`
	BuildB           int               `json:"buildB"`
	Regressions      []TestComparison  `json:"regressions"`
	Progressions     []TestComparison  `json:"progressions"`
	StillPassing     []TestCase        `json:"stillPassing"`
	StillFailing     []TestCase        `json:"stillFailing"`
	NewTests         []TestCase        `json:"newTests"`
	RemovedTests     []TestCase        `json:"removedTests"`
	ChangeCandidates []CommitGroup     `json:"changeCandidates"`
	Summary          ComparisonSummary `json:"summary"`
}

// TestComparison captures status and timing deltas for a test between two builds.
type TestComparison struct {
	TestName    string  `json:"testName"`
	ClassName   string  `json:"className"`
	StatusA     string  `json:"statusA"`
	StatusB     string  `json:"statusB"`
	DurationA   float64 `json:"durationA"`
	DurationB   float64 `json:"durationB"`
	Error       string  `json:"error,omitempty"`
	FailedSince int     `json:"failedSince,omitempty"`
	Age         int     `json:"age,omitempty"`
}

// CommitGroup groups commits by author for change candidate reporting.
type CommitGroup struct {
	Author      string   `json:"author"`
	Commits     []Commit `json:"commits"`
	CommitCount int      `json:"commitCount"`
}

// Commit represents a single SCM change associated with a build.
type Commit struct {
	CommitID  string `json:"commitId"`
	ShortID   string `json:"shortId"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// ComparisonSummary provides aggregate metrics for build comparison views.
type ComparisonSummary struct {
	TotalTestsA       int     `json:"totalTestsA"`
	TotalTestsB       int     `json:"totalTestsB"`
	RegressionsCount  int     `json:"regressionsCount"`
	ProgressionsCount int     `json:"progressionsCount"`
	NewTestsCount     int     `json:"newTestsCount"`
	RemovedTestsCount int     `json:"removedTestsCount"`
	PassRateA         float64 `json:"passRateA"`
	PassRateB         float64 `json:"passRateB"`
	PassRateDelta     float64 `json:"passRateDelta"`
	AvgDurationA      float64 `json:"avgDurationA"`
	AvgDurationB      float64 `json:"avgDurationB"`
	TotalCommits      int     `json:"totalCommits"`
	UniqueAuthors     int     `json:"uniqueAuthors"`
}

// TestTrends aggregates pass/fail trends across builds for charting.
type TestTrends struct {
	Builds     []BuildTrendPoint `json:"builds"`
	TotalCount int               `json:"totalCount"`
}

// BuildTrendPoint represents a single data point for the trend view.
type BuildTrendPoint struct {
	BuildNumber int     `json:"buildNumber"`
	Timestamp   int64   `json:"timestamp"`
	PassCount   int     `json:"passCount"`
	FailCount   int     `json:"failCount"`
	SkipCount   int     `json:"skipCount"`
	TotalCount  int     `json:"totalCount"`
	PassRate    float64 `json:"passRate"`
	Duration    float64 `json:"duration"`
	Result      string  `json:"result"`
}

// AutomationChangeSet represents Jenkins changeset payload structure.
type AutomationChangeSet struct {
	Items []AutomationChangeSetItem `json:"items"`
}

// AutomationChangeSetItem represents an individual commit in Jenkins changeSet API.
type AutomationChangeSetItem struct {
	Author struct {
		FullName string `json:"fullName"`
	} `json:"author"`
	CommitID  string `json:"commitId"`
	Msg       string `json:"msg"`
	Timestamp int64  `json:"timestamp"`
}
