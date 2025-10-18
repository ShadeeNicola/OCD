package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"app/internal/config"
	"app/internal/jenkins/errors"
	"app/internal/jenkins/types"
)

const automationServiceName = "automation"

// automationService implements AutomationService
type automationService struct {
	configuration *config.Config
	client        JenkinsClient
	jobPath       string
}

// NewAutomationService creates a new automation service instance using the configured default job path.
func NewAutomationService(configuration *config.Config, client JenkinsClient) AutomationService {
	return (&automationService{
		configuration: configuration,
		client:        client,
	}).WithJobPath("")
}

// WithJobPath returns a clone of the service targeting the provided Jenkins job path.
func (s *automationService) WithJobPath(jobPath string) AutomationService {
	clone := *s
	clone.jobPath = strings.TrimSpace(jobPath)
	return &clone
}

// GetTestReport retrieves Jenkins test report details for a build.
func (s *automationService) GetTestReport(ctx context.Context, buildNumber int) (*types.TestReport, error) {
	if buildNumber <= 0 {
		return nil, errors.NewInvalidParametersError(
			automationServiceName,
			fmt.Sprintf("invalid build number: %d", buildNumber),
			nil,
		)
	}

	reportURL := fmt.Sprintf("%s/%d/testReport/api/json", s.jobBaseURL(), buildNumber)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	body, err := s.client.GetWithAuth(ctx, reportURL)
	if err != nil {
		return nil, errors.NewJobNotFoundError(
			automationServiceName,
			fmt.Sprintf("failed to fetch test report for build #%d", buildNumber),
			err,
		)
	}

	var raw struct {
		Duration  float64 `json:"duration"`
		PassCount int     `json:"passCount"`
		FailCount int     `json:"failCount"`
		SkipCount int     `json:"skipCount"`
		Suites    []struct {
			Name     string  `json:"name"`
			Duration float64 `json:"duration"`
			Cases    []struct {
				ClassName       string  `json:"className"`
				Name            string  `json:"name"`
				Status          string  `json:"status"`
				Duration        float64 `json:"duration"`
				ErrorDetails    string  `json:"errorDetails"`
				ErrorStackTrace string  `json:"errorStackTrace"`
				FailedSince     int     `json:"failedSince"`
				Age             int     `json:"age"`
			} `json:"cases"`
		} `json:"suites"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.NewParsingError(
			reportURL,
			"failed to parse Jenkins test report response",
			err,
		)
	}

	report := &types.TestReport{
		BuildNumber: buildNumber,
		PassCount:   raw.PassCount,
		FailCount:   raw.FailCount,
		SkipCount:   raw.SkipCount,
		Duration:    raw.Duration,
		Suites:      make([]types.TestSuite, 0, len(raw.Suites)),
	}

	for _, suite := range raw.Suites {
		cases := make([]types.TestCase, 0, len(suite.Cases))
		for _, c := range suite.Cases {
			cases = append(cases, types.TestCase{
				ClassName:       c.ClassName,
				Name:            c.Name,
				Status:          c.Status,
				Duration:        c.Duration,
				ErrorDetails:    strings.TrimSpace(c.ErrorDetails),
				ErrorStackTrace: strings.TrimSpace(c.ErrorStackTrace),
				FailedSince:     c.FailedSince,
				Age:             c.Age,
			})
		}
		report.Suites = append(report.Suites, types.TestSuite{
			Name:     suite.Name,
			Duration: suite.Duration,
			Cases:    cases,
		})
	}

	return report, nil
}

// GetBuildList returns recent build metadata for the automation job.
func (s *automationService) GetBuildList(ctx context.Context, limit int) ([]types.AutomationBuildInfo, error) {
	if limit <= 0 {
		limit = 20
	}

	apiURL := fmt.Sprintf("%s/api/json?tree=builds[number,result,duration,timestamp]{0,%d}", s.jobBaseURL(), limit)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	body, err := s.client.GetWithAuth(ctx, apiURL)
	if err != nil {
		return nil, errors.NewJobNotFoundError(
			automationServiceName,
			"failed to fetch build list",
			err,
		)
	}

	var raw struct {
		Builds []struct {
			Number    int    `json:"number"`
			Result    string `json:"result"`
			Duration  int64  `json:"duration"`
			Timestamp int64  `json:"timestamp"`
		} `json:"builds"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.NewParsingError(
			apiURL,
			"failed to parse build list response",
			err,
		)
	}

	builds := make([]types.AutomationBuildInfo, 0, len(raw.Builds))
	for _, b := range raw.Builds {
		builds = append(builds, types.AutomationBuildInfo{
			Number:    b.Number,
			Result:    b.Result,
			Duration:  b.Duration,
			Timestamp: b.Timestamp,
		})
	}

	return builds, nil
}

// CompareBuilds analyses test results between two builds and returns detailed comparison data.
func (s *automationService) CompareBuilds(ctx context.Context, buildA, buildB int) (*types.BuildComparison, error) {
	if buildA <= 0 || buildB <= 0 {
		return nil, errors.NewInvalidParametersError(
			automationServiceName,
			"build numbers must be greater than zero",
			nil,
		)
	}

	if buildA == buildB {
		return nil, errors.NewInvalidParametersError(
			automationServiceName,
			"build numbers must be different",
			nil,
		)
	}

	reportA, err := s.GetTestReport(ctx, buildA)
	if err != nil {
		return nil, err
	}

	reportB, err := s.GetTestReport(ctx, buildB)
	if err != nil {
		return nil, err
	}

	comparison := &types.BuildComparison{
		BuildA:       buildA,
		BuildB:       buildB,
		Regressions:  []types.TestComparison{},
		Progressions: []types.TestComparison{},
		StillPassing: []types.TestCase{},
		StillFailing: []types.TestCase{},
		NewTests:     []types.TestCase{},
		RemovedTests: []types.TestCase{},
	}

	testsA := flattenTestCases(reportA)
	testsB := flattenTestCases(reportB)

	for key, testB := range testsB {
		testA, existed := testsA[key]
		switch {
		case existed && isFailedStatus(testA.Status) && isPassedStatus(testB.Status):
			comparison.Progressions = append(comparison.Progressions, makeComparison(testA, testB))
		case existed && isPassedStatus(testA.Status) && isFailedStatus(testB.Status):
			comparison.Regressions = append(comparison.Regressions, makeComparison(testA, testB))
		case existed && isFailedStatus(testA.Status) && isFailedStatus(testB.Status):
			comparison.StillFailing = append(comparison.StillFailing, testB)
		case existed && isPassedStatus(testA.Status) && isPassedStatus(testB.Status):
			comparison.StillPassing = append(comparison.StillPassing, testB)
		case !existed:
			comparison.NewTests = append(comparison.NewTests, testB)
		}
	}

	for key, testA := range testsA {
		if _, exists := testsB[key]; !exists {
			comparison.RemovedTests = append(comparison.RemovedTests, testA)
		}
	}

	changeGroups, err := s.GetChangesBetweenBuilds(ctx, buildA, buildB)
	if err == nil {
		comparison.ChangeCandidates = changeGroups
	}

	comparison.Summary = buildComparisonSummary(reportA, reportB, comparison)

	return comparison, nil
}

// GetTestTrends returns trend data for recent builds.
func (s *automationService) GetTestTrends(ctx context.Context, numBuilds int) (*types.TestTrends, error) {
	if numBuilds <= 0 {
		numBuilds = 10
	}

	builds, err := s.GetBuildList(ctx, numBuilds)
	if err != nil {
		return nil, err
	}

	trends := &types.TestTrends{
		Builds:     []types.BuildTrendPoint{},
		TotalCount: 0,
	}

	for _, build := range builds {
		report, err := s.GetTestReport(ctx, build.Number)
		if err != nil {
			continue
		}

		total := report.PassCount + report.FailCount + report.SkipCount
		if total == 0 {
			continue
		}

		trends.Builds = append(trends.Builds, types.BuildTrendPoint{
			BuildNumber: build.Number,
			Timestamp:   build.Timestamp,
			PassCount:   report.PassCount,
			FailCount:   report.FailCount,
			SkipCount:   report.SkipCount,
			TotalCount:  total,
			PassRate:    calculatePassRate(report.PassCount, report.FailCount, report.SkipCount),
			Duration:    report.Duration,
			Result:      build.Result,
		})
	}

	sort.Slice(trends.Builds, func(i, j int) bool {
		return trends.Builds[i].BuildNumber > trends.Builds[j].BuildNumber
	})

	trends.TotalCount = len(trends.Builds)

	return trends, nil
}

// GetChangesBetweenBuilds gathers change-set information between two builds.
func (s *automationService) GetChangesBetweenBuilds(ctx context.Context, buildA, buildB int) ([]types.CommitGroup, error) {
	if buildA <= 0 || buildB <= 0 {
		return nil, errors.NewInvalidParametersError(
			automationServiceName,
			"build numbers must be greater than zero",
			nil,
		)
	}

	if buildB <= buildA {
		return nil, errors.NewInvalidParametersError(
			automationServiceName,
			"buildB must be greater than buildA",
			nil,
		)
	}

	commitMap := make(map[string]*types.CommitGroup)

	for build := buildA + 1; build <= buildB; build++ {
		items, err := s.fetchBuildChangeItems(ctx, build)
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			author := strings.TrimSpace(item.Author.FullName)
			if author == "" {
				author = "Unknown"
			}

			message := strings.TrimSpace(item.Msg)
			if idx := strings.IndexByte(message, '\n'); idx >= 0 {
				message = message[:idx]
			}

			shortID := item.CommitID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}

			commit := types.Commit{
				CommitID:  item.CommitID,
				ShortID:   shortID,
				Message:   message,
				Timestamp: item.Timestamp,
			}

			group, exists := commitMap[author]
			if !exists {
				group = &types.CommitGroup{
					Author:  author,
					Commits: []types.Commit{},
				}
				commitMap[author] = group
			}

			group.Commits = append(group.Commits, commit)
			group.CommitCount = len(group.Commits)
		}
	}

	groups := make([]types.CommitGroup, 0, len(commitMap))
	for _, group := range commitMap {
		sort.Slice(group.Commits, func(i, j int) bool {
			return group.Commits[i].Timestamp > group.Commits[j].Timestamp
		})
		groups = append(groups, *group)
	}

	sort.Slice(groups, func(i, j int) bool {
		return strings.ToLower(groups[i].Author) < strings.ToLower(groups[j].Author)
	})

	return groups, nil
}

// fetchBuildChangeItems retrieves change-set items for a specific build.
func (s *automationService) fetchBuildChangeItems(ctx context.Context, buildNumber int) ([]types.AutomationChangeSetItem, error) {
	changeURL := fmt.Sprintf("%s/%d/api/json?tree=changeSet[items[author[fullName],commitId,msg,timestamp]],changeSets[items[author[fullName],commitId,msg,timestamp]]", s.jobBaseURL(), buildNumber)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	body, err := s.client.GetWithAuth(ctx, changeURL)
	if err != nil {
		return nil, errors.NewJobNotFoundError(
			automationServiceName,
			fmt.Sprintf("failed to fetch change set for build #%d", buildNumber),
			err,
		)
	}

	var raw struct {
		ChangeSet struct {
			Items []types.AutomationChangeSetItem `json:"items"`
		} `json:"changeSet"`
		ChangeSets []struct {
			Items []types.AutomationChangeSetItem `json:"items"`
		} `json:"changeSets"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.NewParsingError(
			changeURL,
			"failed to parse change-set response",
			err,
		)
	}

	items := make([]types.AutomationChangeSetItem, 0, len(raw.ChangeSet.Items))
	items = append(items, raw.ChangeSet.Items...)

	for _, cs := range raw.ChangeSets {
		items = append(items, cs.Items...)
	}

	return items, nil
}

// jobBaseURL returns the Jenkins job base URL, normalising paths as needed.
func (s *automationService) jobBaseURL() string {
	base := strings.TrimSuffix(s.configuration.Endpoints.CustomizationJenkinsBaseURL, "/")
	path := s.jobPath
	if path == "" {
		path = s.configuration.Endpoints.AutomationJenkinsJobPath
	}
	return base + config.NormalizeJobPath(path)
}

func flattenTestCases(report *types.TestReport) map[string]types.TestCase {
	result := make(map[string]types.TestCase)
	for _, suite := range report.Suites {
		for _, test := range suite.Cases {
			key := fmt.Sprintf("%s::%s", test.ClassName, test.Name)
			result[key] = test
		}
	}
	return result
}

func makeComparison(testA, testB types.TestCase) types.TestComparison {
	return types.TestComparison{
		TestName:    testB.Name,
		ClassName:   testB.ClassName,
		StatusA:     strings.ToUpper(testA.Status),
		StatusB:     strings.ToUpper(testB.Status),
		DurationA:   testA.Duration,
		DurationB:   testB.Duration,
		Error:       testB.ErrorDetails,
		FailedSince: testB.FailedSince,
		Age:         testB.Age,
	}
}

func buildComparisonSummary(reportA, reportB *types.TestReport, comparison *types.BuildComparison) types.ComparisonSummary {
	testsA := flattenTestCases(reportA)
	testsB := flattenTestCases(reportB)

	passRateA := calculatePassRate(reportA.PassCount, reportA.FailCount, reportA.SkipCount)
	passRateB := calculatePassRate(reportB.PassCount, reportB.FailCount, reportB.SkipCount)

	avgDurationA := averageTestDuration(testsA)
	avgDurationB := averageTestDuration(testsB)

	uniqueAuthors := make(map[string]struct{})
	totalCommits := 0
	for _, group := range comparison.ChangeCandidates {
		if group.Author != "" {
			uniqueAuthors[group.Author] = struct{}{}
		}
		totalCommits += len(group.Commits)
	}

	return types.ComparisonSummary{
		TotalTestsA:       len(testsA),
		TotalTestsB:       len(testsB),
		RegressionsCount:  len(comparison.Regressions),
		ProgressionsCount: len(comparison.Progressions),
		NewTestsCount:     len(comparison.NewTests),
		RemovedTestsCount: len(comparison.RemovedTests),
		PassRateA:         passRateA,
		PassRateB:         passRateB,
		PassRateDelta:     passRateB - passRateA,
		AvgDurationA:      avgDurationA,
		AvgDurationB:      avgDurationB,
		TotalCommits:      totalCommits,
		UniqueAuthors:     len(uniqueAuthors),
	}
}

func calculatePassRate(passed, failed, skipped int) float64 {
	total := passed + failed + skipped
	if total == 0 {
		return 0
	}
	return (float64(passed) / float64(total)) * 100
}

func averageTestDuration(tests map[string]types.TestCase) float64 {
	if len(tests) == 0 {
		return 0
	}

	total := 0.0
	for _, test := range tests {
		total += test.Duration
	}

	return total / float64(len(tests))
}

func isFailedStatus(status string) bool {
	switch strings.ToUpper(status) {
	case "FAILED", "REGRESSION":
		return true
	default:
		return false
	}
}

func isPassedStatus(status string) bool {
	return strings.ToUpper(status) == "PASSED"
}
