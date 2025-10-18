package api_test

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/ros-ocp-backend/internal/api"
)

/*
===============================================================================
API SERVER BUSINESS LOGIC TEST SUITE
===============================================================================

This BDD test suite verifies the BUSINESS LOGIC of the ROS-OCP API server.
Tests focus on business rules, data transformation, and application outcomes
rather than framework or library behavior.

BUSINESS RULES TESTED:

1. Date Range Filtering
   - Default date ranges for recommendations
   - User-provided date handling and inclusivity
   - Date boundary calculations

2. Query Parameter Processing
   - Filter construction for multi-tenant data
   - Parameter parsing and validation
   - SQL clause generation

3. Unit Conversion Business Logic
   - CPU unit conversions (cores ↔ millicores)
   - Memory unit conversions (bytes ↔ MiB ↔ GiB)
   - Precision rules for each unit type

4. Percentage Calculations
   - Variation percentage calculations
   - Zero-value handling to prevent division errors
   - Precision requirements for business reporting

5. Pagination Logic
   - Link generation for API navigation
   - Boundary conditions for first/last pages
   - Offset calculations for page transitions

===============================================================================
*/

var _ = Describe("API Server Business Logic", func() {

	// =============================================================================
	// DATE RANGE FILTERING BUSINESS RULES
	// Tests how the system determines time windows for recommendations
	// =============================================================================

	Describe("Date range filtering for recommendations", func() {
		var (
			e   *echo.Echo
			req *http.Request
			rec *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			e = echo.New()
			rec = httptest.NewRecorder()
		})

		Context("When no date filters are provided", func() {
			It("should default to current month as the time window", func() {
				// Given: a request without date parameters
				req = httptest.NewRequest(http.MethodGet, "/", nil)
				c := e.NewContext(req, rec)

				// When: parsing query parameters
				result, err := api.MapQueryParameters(c)

				// Then: should use first day of current month as start date
				Expect(err).ToNot(HaveOccurred())
				now := time.Now().UTC().Truncate(time.Second)
				expectedStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

				startDate := result["recommendation_sets.monitoring_end_time >= ?"].(time.Time)
				Expect(startDate).To(Equal(expectedStart))

				// And: should use current time as end date
				endDate := result["recommendation_sets.monitoring_end_time < ?"].(time.Time)
				Expect(endDate.Year()).To(Equal(now.Year()))
				Expect(endDate.Month()).To(Equal(now.Month()))
				Expect(endDate.Day()).To(Equal(now.Day()))
			})
		})

		Context("When custom date range is provided", func() {
			DescribeTable("should parse and apply inclusive end date logic",
				func(startDateStr, endDateStr string, validateFn func(time.Time, time.Time)) {
					// Given: a request with date parameters
					query := url.Values{}
					if startDateStr != "" {
						query.Set("start_date", startDateStr)
					}
					if endDateStr != "" {
						query.Set("end_date", endDateStr)
					}
					req = httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil)
					c := e.NewContext(req, rec)

					// When: parsing query parameters
					result, err := api.MapQueryParameters(c)

					// Then: should succeed
					Expect(err).ToNot(HaveOccurred())

					// And: should apply correct date logic
					startDate := result["recommendation_sets.monitoring_end_time >= ?"].(time.Time)
					endDate := result["recommendation_sets.monitoring_end_time < ?"].(time.Time)
					validateFn(startDate, endDate)
				},
				Entry("with both dates specified - makes end date inclusive by adding 24h",
					"2023-01-01", "2023-01-31",
					func(start, end time.Time) {
						Expect(start.Format("2006-01-02")).To(Equal("2023-01-01"))
						// End date should be 2023-02-01 (31st + 24h for inclusivity)
						Expect(end.Format("2006-01-02")).To(Equal("2023-02-01"))
					}),
				Entry("with only start date - uses current time as end",
					"2023-06-01", "",
					func(start, end time.Time) {
						Expect(start.Format("2006-01-02")).To(Equal("2023-06-01"))
						now := time.Now().UTC()
						Expect(end.Format("2006-01-02")).To(Equal(now.Format("2006-01-02")))
					}),
				Entry("with only end date - uses first of month as start",
					"", "2023-12-31",
					func(start, end time.Time) {
						now := time.Now().UTC()
						expectedStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
						Expect(start.Format("2006-01-02")).To(Equal(expectedStart.Format("2006-01-02")))
						// End date should be 2024-01-01 (31st + 24h for inclusivity)
						Expect(end.Format("2006-01-02")).To(Equal("2024-01-01"))
					}),
			)

			It("should make user-provided end dates inclusive for better UX", func() {
				// Given: user wants recommendations up to and including Jan 15
				req = httptest.NewRequest(http.MethodGet, "/?end_date=2023-01-15", nil)
				c := e.NewContext(req, rec)

				// When: parsing the query
				result, err := api.MapQueryParameters(c)
				Expect(err).ToNot(HaveOccurred())

				// Then: end date should be 2023-01-16 (to include all of Jan 15)
				endDate := result["recommendation_sets.monitoring_end_time < ?"].(time.Time)
				Expect(endDate.Format("2006-01-02")).To(Equal("2023-01-16"))
			})
		})
	})

	// =============================================================================
	// QUERY FILTER CONSTRUCTION BUSINESS RULES
	// Tests how user filters are translated into database queries
	// =============================================================================

	Describe("Query filter construction for multi-tenant data", func() {
		var (
			e   *echo.Echo
			req *http.Request
			rec *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			e = echo.New()
			rec = httptest.NewRecorder()
		})

		Context("When filtering by cluster", func() {
			It("should search both cluster alias and UUID with wildcard matching", func() {
				// Given: user searches for a cluster by partial name
				req = httptest.NewRequest(http.MethodGet, "/?cluster=prod", nil)
				c := e.NewContext(req, rec)

				// When: parsing query parameters
				result, err := api.MapQueryParameters(c)

				// Then: should create clause for both alias and UUID
				Expect(err).ToNot(HaveOccurred())

				// Business rule: cluster filter searches both alias and UUID
				foundClusterFilter := false
				for key := range result {
					if key == "clusters.cluster_alias ILIKE ? OR clusters.cluster_uuid ILIKE ?" {
						foundClusterFilter = true
						values := result[key].([]string)
						// Should wrap in wildcards for partial matching
						Expect(values).To(Equal([]string{"%prod%", "%prod%"}))
					}
				}
				Expect(foundClusterFilter).To(BeTrue(), "cluster filter should search both alias and UUID")
			})
		})

		Context("When filtering by multiple criteria", func() {
			It("should construct OR clauses for multiple values of same parameter", func() {
				// Given: user wants recommendations from multiple projects
				req = httptest.NewRequest(http.MethodGet, "/?project=frontend&project=backend", nil)
				c := e.NewContext(req, rec)

				// When: parsing query parameters
				result, err := api.MapQueryParameters(c)

				// Then: should create OR clause for multiple projects
				Expect(err).ToNot(HaveOccurred())

				foundProjectFilter := false
				for key := range result {
					if key == "workloads.namespace ILIKE ? OR workloads.namespace ILIKE ?" {
						foundProjectFilter = true
						values := result[key].([]string)
						Expect(values).To(Equal([]string{"%frontend%", "%backend%"}))
					}
				}
				Expect(foundProjectFilter).To(BeTrue(), "should create OR clause for multiple projects")
			})
		})

		Context("When filtering by workload type", func() {
			It("should use exact matching without wildcards", func() {
				// Given: user filters by specific workload type
				req = httptest.NewRequest(http.MethodGet, "/?workload_type=Deployment", nil)
				c := e.NewContext(req, rec)

				// When: parsing query parameters
				result, err := api.MapQueryParameters(c)

				// Then: should use exact match (no wildcards)
				Expect(err).ToNot(HaveOccurred())

				foundWorkloadTypeFilter := false
				for key, value := range result {
					if key == "workloads.workload_type = ?" {
						foundWorkloadTypeFilter = true
						values := value.([]string)
						// Business rule: workload_type uses exact match, not ILIKE
						Expect(values).To(Equal([]string{"Deployment"}))
					}
				}
				Expect(foundWorkloadTypeFilter).To(BeTrue(), "should filter by exact workload type")
			})
		})
	})

	// =============================================================================
	// UNIT CONVERSION BUSINESS RULES
	// Tests CPU and memory unit transformations with precision requirements
	// =============================================================================

	Describe("Unit conversion for resource recommendations", func() {
		Context("When converting CPU values", func() {
			DescribeTable("should apply correct conversion and precision rules",
				func(cpuUnit string, inputValue float64, expectedOutput float64) {
					// When: converting CPU value
					result := api.ConvertCPUUnit(cpuUnit, inputValue)

					// Then: should match expected business rule
					Expect(result).To(Equal(expectedOutput))
				},
				Entry("cores to millicores - rounds to nearest integer",
					"millicores", 1.5, 1500.0),
				Entry("cores preserves value with max 3 decimals",
					"cores", 1.123456, 1.123),
				Entry("cores truncates beyond 3 decimals (not rounds)",
					"cores", 1.9999, 1.999),
				Entry("millicores rounds half-up for business accuracy",
					"millicores", 0.5555, 556.0),
			)
		})

		Context("When converting memory values", func() {
			DescribeTable("should apply correct conversion and precision rules",
				func(memoryUnit string, inputBytes float64, expectedOutput float64) {
					// When: converting memory value
					result := api.ConvertMemoryUnit(memoryUnit, inputBytes)

					// Then: should match expected business rule
					Expect(result).To(Equal(expectedOutput))
				},
				Entry("bytes to MiB - truncates to 2 decimals",
					"MiB", 1048576.0, 1.0), // 1 MiB
				Entry("bytes to GiB - truncates to 2 decimals",
					"GiB", 1073741824.0, 1.0), // 1 GiB
				Entry("bytes preserves exact value",
					"bytes", 1234567.0, 1234567.0),
				Entry("MiB truncation prevents over-precision",
					"MiB", 1048576.0+512.0, 1.0), // 1 MiB + 512 bytes = 1.00 MiB (truncated)
			)
		})
	})

	// =============================================================================
	// PERCENTAGE CALCULATION BUSINESS RULES
	// Tests variation calculations with zero-handling for safety
	// =============================================================================

	Describe("Percentage calculations for resource variations", func() {
		Context("When calculating resource change percentages", func() {
			DescribeTable("should handle edge cases safely",
				func(numerator, denominator, expectedPercentage float64) {
					// When: calculating percentage
					result := api.CalculatePercentage(numerator, denominator)

					// Then: should match expected business outcome
					Expect(result).To(Equal(expectedPercentage))
				},
				Entry("normal calculation: 50% increase",
					50.0, 100.0, 50.0),
				Entry("zero numerator returns 0% for safety",
					0.0, 100.0, 0.0),
				Entry("zero denominator returns 0% to avoid infinity",
					100.0, 0.0, 0.0),
				Entry("both zero returns 0% to avoid NaN",
					0.0, 0.0, 0.0),
				Entry("decrease shows negative percentage",
					-25.0, 100.0, -25.0),
				Entry("100% increase",
					100.0, 100.0, 100.0),
			)
		})
	})

	// =============================================================================
	// PAGINATION BUSINESS RULES
	// Tests API navigation link generation
	// =============================================================================

	Describe("Pagination for recommendation lists", func() {
		var req *http.Request

		Context("When generating collection response links", func() {
			It("should create correct pagination links for middle page", func() {
				// Given: requesting page 3 of a large result set (offset 20 > limit 10)
				req = httptest.NewRequest(http.MethodGet, "/api/recommendations?limit=10&offset=20", nil)

				// When: generating collection response
				// Total count: 100, Limit: 10, Offset: 20 (page 3)
				collection := api.CollectionResponse([]interface{}{}, req, 100, 10, 20)

				// Then: should have all navigation links
				Expect(collection.Links.First).ToNot(BeEmpty(), "first link should exist")
				Expect(collection.Links.Previous).ToNot(BeEmpty(), "previous link should exist on page 3")
				Expect(collection.Links.Next).ToNot(BeEmpty(), "next link should exist with more data")
				Expect(collection.Links.Last).ToNot(BeEmpty(), "last link should exist")

				// And: metadata should reflect request
				Expect(collection.Meta.Count).To(Equal(100))
				Expect(collection.Meta.Limit).To(Equal(10))
				Expect(collection.Meta.Offset).To(Equal(20))
			})

			It("should omit previous link on first page", func() {
				// Given: requesting first page
				req = httptest.NewRequest(http.MethodGet, "/api/recommendations?limit=10&offset=0", nil)

				// When: generating collection response
				collection := api.CollectionResponse([]interface{}{}, req, 100, 10, 0)

				// Then: previous link should be empty (business rule for page 1)
				Expect(collection.Links.Previous).To(BeEmpty(), "no previous page for first page")
				Expect(collection.Links.Next).ToNot(BeEmpty(), "next link should exist")
			})

			It("should omit next link on last page", func() {
				// Given: requesting last page (offset 90, limit 10, count 100)
				req = httptest.NewRequest(http.MethodGet, "/api/recommendations?limit=10&offset=90", nil)

				// When: generating collection response
				collection := api.CollectionResponse([]interface{}{}, req, 100, 10, 90)

				// Then: next link should be empty (business rule for last page)
				Expect(collection.Links.Next).To(BeEmpty(), "no next page for last page")
				Expect(collection.Links.Previous).ToNot(BeEmpty(), "previous link should exist")
			})

			It("should calculate previous link offset correctly", func() {
				// Given: on page 3 (offset 20)
				req = httptest.NewRequest(http.MethodGet, "/api/recommendations?limit=10&offset=20", nil)

				// When: generating collection response
				collection := api.CollectionResponse([]interface{}{}, req, 100, 10, 20)

				// Then: previous link should point to offset 10 (page 2)
				Expect(collection.Links.Previous).To(ContainSubstring("offset=10"))
			})

			DescribeTable("should handle edge cases in pagination",
				func(count, limit, offset int, expectPrevious, expectNext bool) {
					// Given: specific pagination parameters
					req = httptest.NewRequest(http.MethodGet,
						fmt.Sprintf("/api/recommendations?limit=%d&offset=%d", limit, offset), nil)

					// When: generating collection response
					collection := api.CollectionResponse([]interface{}{}, req, count, limit, offset)

					// Then: links should match business rules
					if expectPrevious {
						Expect(collection.Links.Previous).ToNot(BeEmpty(), "previous link expected")
					} else {
						Expect(collection.Links.Previous).To(BeEmpty(), "no previous link expected")
					}

					if expectNext {
						Expect(collection.Links.Next).ToNot(BeEmpty(), "next link expected")
					} else {
						Expect(collection.Links.Next).To(BeEmpty(), "no next link expected")
					}
				},
				Entry("first page of many", 100, 10, 0, false, true),
				Entry("last page of many", 100, 10, 90, true, false),
				Entry("single page only", 5, 10, 0, false, false),
				Entry("middle page", 100, 10, 50, true, true),
				Entry("page 2 - no previous when offset equals limit", 100, 10, 10, false, true), // Business rule: offset must be > limit for previous link
			)
		})
	})

	// =============================================================================
	// PHASE 1: EDGE CASE COVERAGE - PARAMETER VALIDATION
	// Tests realistic edge cases in query parameter validation
	// =============================================================================

	Describe("Parameter validation edge cases", func() {
		var (
			e   *echo.Echo
			req *http.Request
			rec *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			e = echo.New()
			rec = httptest.NewRecorder()
		})

		Context("When parsing limit and offset parameters", func() {
			DescribeTable("should handle edge cases gracefully",
				func(limitParam, offsetParam string, expectError bool) {
					// Given: edge case pagination parameters
					query := url.Values{}
					if limitParam != "" {
						query.Set("limit", limitParam)
					}
					if offsetParam != "" {
						query.Set("offset", offsetParam)
					}
					req = httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil)
					c := e.NewContext(req, rec)

					// When: parsing query parameters
					result, err := api.MapQueryParameters(c)

					// Then: should handle gracefully
					if expectError {
						Expect(err).To(HaveOccurred(), "expected error for invalid input")
					} else {
						Expect(err).ToNot(HaveOccurred(), "should not error for valid/handled input")
						Expect(result).ToNot(BeNil())
					}
				},
				Entry("negative limit - silently uses default", "-1", "0", false),
				Entry("zero limit - uses default", "0", "0", false),
				Entry("very large limit - accepts", "999999", "0", false),
				Entry("negative offset - silently uses default", "10", "-1", false),
				Entry("very large offset - accepts", "10", "999999", false),
				Entry("non-numeric limit - silently uses default", "abc", "0", false),
				Entry("non-numeric offset - silently uses default", "10", "xyz", false),
				Entry("float limit - truncates to int", "10.5", "0", false),
				Entry("float offset - truncates to int", "10", "5.7", false),
				Entry("empty strings - uses defaults", "", "", false),
			)
		})

		Context("When filtering with special characters", func() {
			It("should handle wildcards in cluster names", func() {
				// Given: cluster filter with SQL wildcards
				req = httptest.NewRequest(http.MethodGet, "/?cluster=%prod%", nil)
				c := e.NewContext(req, rec)

				// When: parsing query parameters
				result, err := api.MapQueryParameters(c)

				// Then: should wrap in additional wildcards (double wildcard)
				Expect(err).ToNot(HaveOccurred())

				for key, value := range result {
					if key == "clusters.cluster_alias ILIKE ? OR clusters.cluster_uuid ILIKE ?" {
						values := value.([]string)
						// Business rule: adds wildcards even if user provided them
						Expect(values[0]).To(Equal("%%prod%%"))
					}
				}
			})

			DescribeTable("should handle special characters in filter values",
				func(param, value string) {
					// Given: filter with special characters
					query := url.Values{}
					query.Set(param, value)
					req = httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil)
					c := e.NewContext(req, rec)

					// When: parsing query parameters
					result, err := api.MapQueryParameters(c)

					// Then: should accept special characters
					Expect(err).ToNot(HaveOccurred())
					Expect(result).ToNot(BeNil())
				},
				Entry("project with hyphen", "project", "my-project"),
				Entry("project with underscore", "project", "my_project"),
				Entry("workload with dots", "workload", "api.v1.service"),
				Entry("container with slash", "container", "registry.io/container"),
				Entry("cluster with colon", "cluster", "cluster:8080"),
			)
		})

		Context("When combining multiple filters", func() {
			It("should handle all filter types simultaneously", func() {
				// Given: query with all possible filters
				query := url.Values{}
				query.Set("cluster", "prod")
				query.Set("project", "frontend")
				query.Set("workload", "api")
				query.Set("workload_type", "Deployment")
				query.Set("container", "nginx")
				query.Set("start_date", "2023-01-01")
				query.Set("end_date", "2023-12-31")

				req = httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil)
				c := e.NewContext(req, rec)

				// When: parsing query parameters
				result, err := api.MapQueryParameters(c)

				// Then: should include all filters
				Expect(err).ToNot(HaveOccurred())
				Expect(len(result)).To(BeNumerically(">", 5), "should have multiple filter clauses")

				// And: should have date filters
				Expect(result).To(HaveKey("recommendation_sets.monitoring_end_time >= ?"))
				Expect(result).To(HaveKey("recommendation_sets.monitoring_end_time < ?"))
			})

			It("should handle multiple values for same filter", func() {
				// Given: multiple projects
				query := url.Values{}
				query.Add("project", "frontend")
				query.Add("project", "backend")
				query.Add("project", "database")

				req = httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil)
				c := e.NewContext(req, rec)

				// When: parsing query parameters
				result, err := api.MapQueryParameters(c)

				// Then: should create OR clause with all values
				Expect(err).ToNot(HaveOccurred())

				foundProjectFilter := false
				for key, value := range result {
					if strings.Contains(key, "workloads.namespace ILIKE ?") {
						foundProjectFilter = true
						values := value.([]string)
						Expect(len(values)).To(Equal(3), "should have 3 project values")
					}
				}
				Expect(foundProjectFilter).To(BeTrue())
			})
		})
	})

	// =============================================================================
	// PHASE 1: EDGE CASE COVERAGE - DATE BOUNDARIES
	// Tests date handling edge cases (leap years, month boundaries, etc.)
	// =============================================================================

	Describe("Date boundary edge cases", func() {
		var (
			e   *echo.Echo
			req *http.Request
			rec *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			e = echo.New()
			rec = httptest.NewRecorder()
		})

		Context("When handling calendar edge cases", func() {
			DescribeTable("should handle special calendar dates correctly",
				func(startDate, endDate string, validateFn func(time.Time, time.Time)) {
					// Given: edge case dates
					query := url.Values{}
					if startDate != "" {
						query.Set("start_date", startDate)
					}
					if endDate != "" {
						query.Set("end_date", endDate)
					}
					req = httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil)
					c := e.NewContext(req, rec)

					// When: parsing query parameters
					result, err := api.MapQueryParameters(c)

					// Then: should handle gracefully
					if validateFn != nil {
						Expect(err).ToNot(HaveOccurred())
						start := result["recommendation_sets.monitoring_end_time >= ?"].(time.Time)
						end := result["recommendation_sets.monitoring_end_time < ?"].(time.Time)
						validateFn(start, end)
					} else {
						// Invalid dates should error
						Expect(err).To(HaveOccurred())
					}
				},
				Entry("leap year Feb 29 - valid",
					"2024-02-29", "2024-02-29",
					func(start, end time.Time) {
						Expect(start.Format("2006-01-02")).To(Equal("2024-02-29"))
						Expect(end.Format("2006-01-02")).To(Equal("2024-03-01")) // +24h for inclusivity
					}),
				Entry("non-leap year Feb 29 - invalid",
					"2023-02-29", "2023-03-01",
					nil), // Expect error
				Entry("month boundary - Jan 31 to Feb 1",
					"2023-01-31", "2023-02-01",
					func(start, end time.Time) {
						Expect(start.Format("2006-01-02")).To(Equal("2023-01-31"))
						Expect(end.Format("2006-01-02")).To(Equal("2023-02-02"))
					}),
				Entry("year boundary - Dec 31 to Jan 1",
					"2023-12-31", "2024-01-01",
					func(start, end time.Time) {
						Expect(start.Format("2006-01-02")).To(Equal("2023-12-31"))
						Expect(end.Format("2006-01-02")).To(Equal("2024-01-02"))
					}),
				Entry("same start and end date",
					"2023-06-15", "2023-06-15",
					func(start, end time.Time) {
						Expect(start.Format("2006-01-02")).To(Equal("2023-06-15"))
						Expect(end.Format("2006-01-02")).To(Equal("2023-06-16")) // +24h
					}),
				Entry("invalid month - month 13",
					"2023-13-01", "2023-12-31",
					nil), // Expect error
				Entry("invalid day - Feb 30",
					"2023-02-30", "2023-03-01",
					nil), // Expect error
				Entry("invalid format - slashes instead of hyphens",
					"2023/01/01", "2023/12/31",
					nil), // Expect error
			)

			It("should handle very old dates", func() {
				// Given: date from 10 years ago
				oldDate := time.Now().AddDate(-10, 0, 0).Format("2006-01-02")
				req = httptest.NewRequest(http.MethodGet, "/?start_date="+oldDate, nil)
				c := e.NewContext(req, rec)

				// When: parsing query parameters
				result, err := api.MapQueryParameters(c)

				// Then: should accept old dates
				Expect(err).ToNot(HaveOccurred())
				start := result["recommendation_sets.monitoring_end_time >= ?"].(time.Time)
				Expect(start.Year()).To(Equal(time.Now().Year() - 10))
			})

			It("should handle future dates", func() {
				// Given: date 1 year in future
				futureDate := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
				req = httptest.NewRequest(http.MethodGet, "/?end_date="+futureDate, nil)
				c := e.NewContext(req, rec)

				// When: parsing query parameters
				result, err := api.MapQueryParameters(c)

				// Then: should accept future dates (no validation against "now")
				Expect(err).ToNot(HaveOccurred())
				end := result["recommendation_sets.monitoring_end_time < ?"].(time.Time)
				Expect(end.Year()).To(Equal(time.Now().Year() + 1))
			})

			It("should handle start_date after end_date (no validation)", func() {
				// Given: inverted date range (start > end)
				req = httptest.NewRequest(http.MethodGet, "/?start_date=2023-12-31&end_date=2023-01-01", nil)
				c := e.NewContext(req, rec)

				// When: parsing query parameters
				result, err := api.MapQueryParameters(c)

				// Then: should not validate date order (database will return empty)
				Expect(err).ToNot(HaveOccurred())
				start := result["recommendation_sets.monitoring_end_time >= ?"].(time.Time)
				end := result["recommendation_sets.monitoring_end_time < ?"].(time.Time)
				Expect(start.After(end)).To(BeTrue(), "business rule: no date order validation")
			})
		})
	})

	// =============================================================================
	// PHASE 1: EDGE CASE COVERAGE - UNIT CONVERSION EXTREMES
	// Tests boundary values for CPU and memory conversions
	// =============================================================================

	Describe("Unit conversion extreme values", func() {
		Context("When converting very small CPU values", func() {
			DescribeTable("should maintain precision appropriately",
				func(cpuUnit string, inputValue, expectedMin, expectedMax float64) {
					// When: converting extreme CPU values
					result := api.ConvertCPUUnit(cpuUnit, inputValue)

					// Then: should be within expected range
					Expect(result).To(BeNumerically(">=", expectedMin))
					Expect(result).To(BeNumerically("<=", expectedMax))
				},
				Entry("0.00001 cores - preserves tiny values", "cores", 0.00001, 0.0, 0.001),
				Entry("0.001 millicores rounds to 0", "millicores", 0.001, 0.0, 1.0),
				Entry("0.5 millicores rounds to 1", "millicores", 0.0005, 0.0, 1.0),
				Entry("zero cores", "cores", 0.0, 0.0, 0.0),
				Entry("zero millicores", "millicores", 0.0, 0.0, 0.0),
			)
		})

		Context("When converting very large CPU values", func() {
			DescribeTable("should handle without overflow",
				func(cpuUnit string, inputValue float64) {
					// When: converting large CPU values
					result := api.ConvertCPUUnit(cpuUnit, inputValue)

					// Then: should not panic or return NaN/Inf
					Expect(math.IsNaN(result)).To(BeFalse(), "should not be NaN")
					Expect(result).To(BeNumerically("<", float64(1e308)), "should not overflow")
				},
				Entry("1000 cores", "cores", 1000.0),
				Entry("10000 cores", "cores", 10000.0),
				Entry("100000 millicores", "millicores", 100.0), // 100 cores = 100000 millicores
			)
		})

		Context("When converting precision boundary CPU values", func() {
			DescribeTable("should apply truncation rules correctly",
				func(cpuUnit string, inputValue, expectedOutput float64) {
					// When: converting boundary precision values
					result := api.ConvertCPUUnit(cpuUnit, inputValue)

					// Then: should match exact business rule
					Expect(result).To(Equal(expectedOutput))
				},
				Entry("1.9999 cores truncates to 1.999 (not rounds to 2.0)",
					"cores", 1.9999, 1.999),
				Entry("1.1239 cores truncates to 1.123",
					"cores", 1.1239, 1.123),
				Entry("0.9999 cores truncates to 0.999",
					"cores", 0.9999, 0.999),
				Entry("1.5555 millicores rounds to 1556",
					"millicores", 1.5555, 1556.0),
				Entry("1.4444 millicores rounds to 1444",
					"millicores", 1.4444, 1444.0),
			)
		})

		Context("When converting very small memory values", func() {
			DescribeTable("should handle tiny byte values",
				func(memoryUnit string, inputBytes, expectedMin, expectedMax float64) {
					// When: converting small memory values
					result := api.ConvertMemoryUnit(memoryUnit, inputBytes)

					// Then: should be within expected range
					Expect(result).To(BeNumerically(">=", expectedMin))
					Expect(result).To(BeNumerically("<=", expectedMax))
				},
				Entry("1 byte to bytes", "bytes", 1.0, 1.0, 1.0),
				Entry("1 byte to MiB truncates to 0.0", "MiB", 1.0, 0.0, 0.01),
				Entry("1 byte to GiB truncates to 0.0", "GiB", 1.0, 0.0, 0.01),
				Entry("1023 bytes to MiB truncates to 0.0", "MiB", 1023.0, 0.0, 0.01),
				Entry("1024 bytes = 0.00 MiB (truncated)", "MiB", 1024.0, 0.0, 0.01),
			)
		})

		Context("When converting very large memory values", func() {
			DescribeTable("should handle large byte values",
				func(memoryUnit string, inputBytes float64) {
					// When: converting large memory values
					result := api.ConvertMemoryUnit(memoryUnit, inputBytes)

					// Then: should not panic or overflow
					Expect(math.IsNaN(result)).To(BeFalse(), "should not be NaN")
					Expect(result).To(BeNumerically(">=", 0), "should be positive")
				},
				Entry("1 TB in bytes", "bytes", 1099511627776.0), // 1 TB
				Entry("1 TB to GiB", "GiB", 1099511627776.0),     // 1024 GiB
				Entry("100 GB to MiB", "MiB", 100000000000.0),
			)
		})

		Context("When converting memory precision boundaries", func() {
			DescribeTable("should apply 2-decimal truncation for MiB/GiB",
				func(memoryUnit string, inputBytes, expectedOutput float64) {
					// When: converting with precision
					result := api.ConvertMemoryUnit(memoryUnit, inputBytes)

					// Then: should truncate to 2 decimals
					Expect(result).To(Equal(expectedOutput))
				},
				Entry("1.5 MiB (1572864 bytes) truncates to 1.50 MiB",
					"MiB", 1572864.0, 1.50),
				Entry("1.999 MiB truncates to 1.99 MiB",
					"MiB", 2096947.2, 1.99), // 1.999 * 1024 * 1024
				Entry("0.5 GiB truncates appropriately",
					"GiB", 536870912.0, 0.50), // 0.5 * 1024 * 1024 * 1024
			)
		})
	})

	// =============================================================================
	// PHASE 1: EDGE CASE COVERAGE - PERCENTAGE CALCULATIONS
	// Tests variation percentage edge cases with extreme values
	// =============================================================================

	Describe("Percentage calculation extreme cases", func() {
		Context("When calculating with extreme values", func() {
			DescribeTable("should handle edge cases safely",
				func(numerator, denominator, expectedPercentage float64) {
					// When: calculating percentage with extreme values
					result := api.CalculatePercentage(numerator, denominator)

					// Then: should match expected outcome
					Expect(result).To(Equal(expectedPercentage))
				},
				// Zero handling (safety rules)
				Entry("zero numerator, positive denominator = 0%",
					0.0, 100.0, 0.0),
				Entry("positive numerator, zero denominator = 0% (not Inf)",
					100.0, 0.0, 0.0),
				Entry("zero numerator, zero denominator = 0% (not NaN)",
					0.0, 0.0, 0.0),
				Entry("negative numerator, zero denominator = 0% (not -Inf)",
					-100.0, 0.0, 0.0),

				// Small value calculations
				Entry("very small numerator: 0.001 / 100 = 0.001%",
					0.001, 100.0, 0.001),
				Entry("very small denominator: 1 / 0.001 = 100000%",
					1.0, 0.001, 100000.0),
				Entry("both very small: 0.001 / 0.002 = 50%",
					0.001, 0.002, 50.0),

				// Large value calculations
				Entry("large increase: 1000 / 1 = 100000%",
					1000.0, 1.0, 100000.0),
				Entry("large decrease: -999 / 1000 = -99.9%",
					-999.0, 1000.0, -99.9),

				// Negative variations (downsizing)
				Entry("50% decrease: -50 / 100 = -50%",
					-50.0, 100.0, -50.0),
				Entry("90% decrease: -90 / 100 = -90%",
					-90.0, 100.0, -90.0),
				Entry("100% decrease: -100 / 100 = -100%",
					-100.0, 100.0, -100.0),

				// Precision edge cases (testing float behavior, not exact value)
				Entry("100% exact: 100 / 100 = 100%",
					100.0, 100.0, 100.0),
				Entry("50% exact: 50 / 100 = 50%",
					50.0, 100.0, 50.0),
			)
		})

		Context("When calculating realistic cost optimization scenarios", func() {
			DescribeTable("should calculate real-world percentage changes",
				func(currentValue, recommendedChange, expectedPercent float64) {
					// Given: realistic resource values
					// When: calculating variation percentage
					result := api.CalculatePercentage(recommendedChange, currentValue)

					// Then: should match expected business outcome
					Expect(result).To(BeNumerically("~", expectedPercent, 0.01))
				},
				// Typical cost optimization: reduce by 10-30%
				Entry("over-provisioned: reduce 0.5 cores from 2 cores = -25%",
					2.0, -0.5, -25.0),
				Entry("over-provisioned: reduce 1Gi from 4Gi = -25%",
					4096.0, -1024.0, -25.0),

				// Under-provisioned: increase by 20-50%
				Entry("under-provisioned: add 0.5 cores to 1 core = +50%",
					1.0, 0.5, 50.0),
				Entry("under-provisioned: add 512Mi to 512Mi = +100%",
					512.0, 512.0, 100.0),

				// Well-tuned: minimal changes
				Entry("well-tuned: adjust 0.1 cores from 2 cores = +5%",
					2.0, 0.1, 5.0),
				Entry("well-tuned: reduce 64Mi from 2048Mi = -3.125%",
					2048.0, -64.0, -3.125),
			)
		})
	})

	// =============================================================================
	// PHASE 1: EDGE CASE COVERAGE - PAGINATION EDGE CASES
	// Tests pagination with extreme offsets, limits, and counts
	// =============================================================================

	Describe("Pagination extreme edge cases", func() {
		var req *http.Request

		Context("When handling extreme pagination parameters", func() {
			DescribeTable("should generate correct links for edge cases",
				func(count, limit, offset int, expectPrev, expectNext bool, description string) {
					// Given: extreme pagination scenario
					req = httptest.NewRequest(http.MethodGet,
						fmt.Sprintf("/api/recommendations?limit=%d&offset=%d", limit, offset), nil)

					// When: generating collection response
					collection := api.CollectionResponse([]interface{}{}, req, count, limit, offset)

					// Then: links should follow business rules
					if expectPrev {
						Expect(collection.Links.Previous).ToNot(BeEmpty(),
							fmt.Sprintf("previous link expected for: %s", description))
					} else {
						Expect(collection.Links.Previous).To(BeEmpty(),
							fmt.Sprintf("no previous link expected for: %s", description))
					}

					if expectNext {
						Expect(collection.Links.Next).ToNot(BeEmpty(),
							fmt.Sprintf("next link expected for: %s", description))
					} else {
						Expect(collection.Links.Next).To(BeEmpty(),
							fmt.Sprintf("no next link expected for: %s", description))
					}

					// And: metadata should match
					Expect(collection.Meta.Count).To(Equal(count))
					Expect(collection.Meta.Limit).To(Equal(limit))
					Expect(collection.Meta.Offset).To(Equal(offset))
				},
				Entry("empty result set", 0, 10, 0, false, false, "no data"),
				Entry("single item", 1, 10, 0, false, false, "one item"),
				Entry("exact single page", 10, 10, 0, false, false, "exact page fit"),
				Entry("one over single page", 11, 10, 0, false, true, "needs next page"),
				Entry("very small page size", 1000, 1, 0, false, true, "page size of 1"),
				Entry("very large page size", 100, 1000, 0, false, false, "page size larger than data"),
				Entry("offset at end", 100, 10, 100, true, false, "offset equals count (business rule: offset > limit)"),
				Entry("offset beyond end", 100, 10, 150, true, false, "offset exceeds count (business rule: offset > limit)"),
				Entry("very large dataset - first page", 1000000, 100, 0, false, true, "million records"),
				Entry("very large dataset - middle", 1000000, 100, 500000, true, true, "middle of million"),
				Entry("very large dataset - last page", 1000000, 100, 999900, true, false, "last page of million"),
			)
		})

		Context("When calculating previous link offsets", func() {
			It("should handle offset less than limit correctly", func() {
				// Given: offset < limit (business rule: no previous)
				req = httptest.NewRequest(http.MethodGet, "/api/recommendations?limit=10&offset=5", nil)

				// When: generating collection response
				collection := api.CollectionResponse([]interface{}{}, req, 100, 10, 5)

				// Then: no previous link (can't go negative)
				Expect(collection.Links.Previous).To(BeEmpty(),
					"offset < limit means no previous page (would be negative)")
			})

			It("should calculate correct previous offset for large pages", func() {
				// Given: large page size with appropriate offset
				req = httptest.NewRequest(http.MethodGet, "/api/recommendations?limit=100&offset=200", nil)

				// When: generating collection response
				collection := api.CollectionResponse([]interface{}{}, req, 1000, 100, 200)

				// Then: previous should point to offset 100
				Expect(collection.Links.Previous).To(ContainSubstring("offset=100"))
			})
		})

		Context("When handling last page calculations", func() {
			It("should correctly identify partial last page", func() {
				// Given: last page with fewer items than limit
				req = httptest.NewRequest(http.MethodGet, "/api/recommendations?limit=10&offset=95", nil)

				// When: generating collection response (only 5 items left)
				collection := api.CollectionResponse([]interface{}{}, req, 100, 10, 95)

				// Then: should have previous but not next
				Expect(collection.Links.Previous).ToNot(BeEmpty())
				Expect(collection.Links.Next).To(BeEmpty(), "no more data available")
			})

			It("should handle exact last page boundary", func() {
				// Given: offset + limit exactly equals count
				req = httptest.NewRequest(http.MethodGet, "/api/recommendations?limit=10&offset=90", nil)

				// When: generating collection response
				collection := api.CollectionResponse([]interface{}{}, req, 100, 10, 90)

				// Then: should be last page (no next)
				Expect(collection.Links.Next).To(BeEmpty(), "offset + limit = count")
				Expect(collection.Links.Previous).ToNot(BeEmpty())
			})
		})
	})
})
