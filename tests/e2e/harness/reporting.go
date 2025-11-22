package harness

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TestReport represents a test execution report
type TestReport struct {
	TestRunID   string       `json:"test_run_id"`
	Format      string       `json:"format"`
	Path        string       `json:"path"`
	Summary     ReportSummary `json:"summary"`
	TestCases   []TestCase   `json:"test_cases"`
	CreatedAt   time.Time    `json:"created_at"`
}

// ReportSummary contains summary statistics
type ReportSummary struct {
	TotalTests int `json:"total_tests"`
	Passed     int `json:"passed"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
	DurationMs int `json:"duration_ms"`
}

// TestCase represents a single test case result
type TestCase struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Suite       string    `json:"suite"`
	Status      string    `json:"status"`
	DurationMs  int       `json:"duration_ms"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Error       string    `json:"error,omitempty"`
	Steps       []TestStep `json:"steps,omitempty"`
}

// TestStep represents a single test step
type TestStep struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Status       string            `json:"status"`
	DurationMs   int               `json:"duration_ms"`
	Request      *RequestDetails   `json:"request,omitempty"`
	Response     *ResponseDetails  `json:"response,omitempty"`
	CorrelationIDs map[string]string `json:"correlation_ids,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// RequestDetails contains request information
type RequestDetails struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// ResponseDetails contains response information
type ResponseDetails struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string  `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	DurationMs int               `json:"duration_ms"`
}

// Reporter handles test report generation
type Reporter struct {
	reportDir string
	runID     string
}

// NewReporter creates a new reporter
func NewReporter(reportDir, runID string) *Reporter {
	return &Reporter{
		reportDir: reportDir,
		runID:     runID,
	}
}

// GenerateJSONReport generates a JSON test report
func (r *Reporter) GenerateJSONReport(report *TestReport) (string, error) {
	report.Format = "json"
	report.CreatedAt = time.Now()

	filename := filepath.Join(r.reportDir, fmt.Sprintf("test-report-%s.json", r.runID))
	if err := os.MkdirAll(r.reportDir, 0755); err != nil {
		return "", fmt.Errorf("create report directory: %w", err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", fmt.Errorf("write report file: %w", err)
	}

	report.Path = filename
	return filename, nil
}

// GenerateJUnitXMLReport generates a JUnit XML test report
func (r *Reporter) GenerateJUnitXMLReport(report *TestReport) (string, error) {
	report.Format = "junit"
	report.CreatedAt = time.Now()

	filename := filepath.Join(r.reportDir, fmt.Sprintf("test-results-%s.xml", r.runID))
	if err := os.MkdirAll(r.reportDir, 0755); err != nil {
		return "", fmt.Errorf("create report directory: %w", err)
	}

	junit := convertToJUnit(report)

	data, err := xml.MarshalIndent(junit, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal JUnit XML: %w", err)
	}

	// Add XML header
	xmlData := []byte(xml.Header)
	xmlData = append(xmlData, data...)

	if err := os.WriteFile(filename, xmlData, 0644); err != nil {
		return "", fmt.Errorf("write JUnit XML file: %w", err)
	}

	report.Path = filename
	return filename, nil
}

// JUnitTestSuites represents JUnit XML structure
type JUnitTestSuites struct {
	XMLName  xml.Name         `xml:"testsuites"`
	Name     string           `xml:"name,attr"`
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Time     float64          `xml:"time,attr"`
	Suites   []JUnitTestSuite `xml:"testsuite"`
}

// JUnitTestSuite represents a test suite in JUnit XML
type JUnitTestSuite struct {
	XMLName  xml.Name        `xml:"testsuite"`
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Skipped  int             `xml:"skipped,attr"`
	Time     float64         `xml:"time,attr"`
	Cases    []JUnitTestCase `xml:"testcase"`
}

// JUnitTestCase represents a test case in JUnit XML
type JUnitTestCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
	Skipped   *JUnitSkipped `xml:"skipped,omitempty"`
}

// JUnitFailure represents a test failure in JUnit XML
type JUnitFailure struct {
	XMLName xml.Name `xml:"failure"`
	Message string   `xml:"message,attr"`
	Type    string   `xml:"type,attr"`
	Content string   `xml:",chardata"`
}

// JUnitSkipped represents a skipped test in JUnit XML
type JUnitSkipped struct {
	XMLName xml.Name `xml:"skipped"`
	Message string   `xml:"message,attr"`
}

// convertToJUnit converts a TestReport to JUnit XML format
func convertToJUnit(report *TestReport) *JUnitTestSuites {
	suites := make(map[string]*JUnitTestSuite)

	for _, tc := range report.TestCases {
		suite, exists := suites[tc.Suite]
		if !exists {
			suite = &JUnitTestSuite{
				Name: tc.Suite,
				Cases: []JUnitTestCase{},
			}
			suites[tc.Suite] = suite
		}

		testCase := JUnitTestCase{
			Name:      tc.Name,
			ClassName: tc.Suite,
			Time:      float64(tc.DurationMs) / 1000.0,
		}

		if tc.Status == "failed" {
			testCase.Failure = &JUnitFailure{
				Message: tc.Error,
				Type:    "TestFailure",
				Content: tc.Error,
			}
			suite.Failures++
		} else if tc.Status == "skipped" {
			testCase.Skipped = &JUnitSkipped{
				Message: "Test skipped",
			}
			suite.Skipped++
		}

		suite.Cases = append(suite.Cases, testCase)
		suite.Tests++
	}

	junitSuites := []JUnitTestSuite{}
	totalTests := 0
	totalFailures := 0
	totalTime := 0.0

	for _, suite := range suites {
		suite.Time = 0.0
		for _, tc := range suite.Cases {
			suite.Time += tc.Time
		}
		junitSuites = append(junitSuites, *suite)
		totalTests += suite.Tests
		totalFailures += suite.Failures
		totalTime += suite.Time
	}

	return &JUnitTestSuites{
		Name:     "e2e-tests",
		Tests:    totalTests,
		Failures: totalFailures,
		Time:     totalTime,
		Suites:   junitSuites,
	}
}

// ConsoleReporter provides console output for test results
type ConsoleReporter struct{}

// ReportTestCase reports a test case result to console
func (r *ConsoleReporter) ReportTestCase(tc TestCase) {
	status := "PASS"
	if tc.Status == "failed" {
		status = "FAIL"
	} else if tc.Status == "skipped" {
		status = "SKIP"
	}

	fmt.Printf("[%s] %s (%dms)\n", status, tc.Name, tc.DurationMs)
	if tc.Error != "" {
		fmt.Printf("  ERROR: %s\n", tc.Error)
	}
}

// ReportSummary reports summary statistics to console
func (r *ConsoleReporter) ReportSummary(summary ReportSummary) {
	fmt.Printf("\n=== Test Summary ===\n")
	fmt.Printf("Total: %d\n", summary.TotalTests)
	fmt.Printf("Passed: %d\n", summary.Passed)
	fmt.Printf("Failed: %d\n", summary.Failed)
	fmt.Printf("Skipped: %d\n", summary.Skipped)
	fmt.Printf("Duration: %dms\n", summary.DurationMs)
}

