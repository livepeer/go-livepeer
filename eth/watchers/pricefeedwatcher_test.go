package watchers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewTruncatedTicker(t *testing.T) {
	testTimeout := time.After(10 * time.Second)
	require := require.New(t)
	newTime := func(s string) time.Time {
		t, err := time.Parse(time.RFC3339, s)
		require.NoError(err)
		return t
	}
	testCases := []struct {
		Period        time.Duration
		StartTime     time.Time
		ExpectedTicks []time.Time
	}{
		{
			Period:    1 * time.Minute,
			StartTime: newTime("2021-01-01T13:02:15Z"),
			ExpectedTicks: []time.Time{
				newTime("2021-01-01T13:03:00Z"),
				newTime("2021-01-01T13:04:00Z"),
				newTime("2021-01-01T13:05:00Z"),
				newTime("2021-01-01T13:06:00Z"),
				newTime("2021-01-01T13:07:00Z"),
			},
		},
		{
			Period:    5 * time.Minute,
			StartTime: newTime("2021-01-01T16:19:45Z"),
			ExpectedTicks: []time.Time{
				newTime("2021-01-01T16:20:00Z"),
				newTime("2021-01-01T16:25:00Z"),
				newTime("2021-01-01T16:30:00Z"),
				newTime("2021-01-01T16:35:00Z"),
				newTime("2021-01-01T16:40:00Z"),
			},
		},
		{
			Period:    15 * time.Minute,
			StartTime: newTime("2021-01-01T19:51:17Z"),
			ExpectedTicks: []time.Time{
				newTime("2021-01-01T20:00:00Z"),
				newTime("2021-01-01T20:15:00Z"),
				newTime("2021-01-01T20:30:00Z"),
				newTime("2021-01-01T20:45:00Z"),
				newTime("2021-01-01T21:00:00Z"),
			},
		},
		{
			Period:    1 * time.Hour,
			StartTime: newTime("2021-01-01T22:30:00Z"),
			ExpectedTicks: []time.Time{
				newTime("2021-01-01T23:00:00Z"),
				newTime("2021-01-02T00:00:00Z"),
				newTime("2021-01-02T01:00:00Z"),
				newTime("2021-01-02T02:00:00Z"),
				newTime("2021-01-02T03:00:00Z"),
			},
		},
		{
			Period:    4 * time.Hour,
			StartTime: newTime("2021-01-02T03:00:00Z"),
			ExpectedTicks: []time.Time{
				newTime("2021-01-02T04:00:00Z"),
				newTime("2021-01-02T08:00:00Z"),
				newTime("2021-01-02T12:00:00Z"),
				newTime("2021-01-02T16:00:00Z"),
				newTime("2021-01-02T20:00:00Z"),
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Period.String(), func(t *testing.T) {
			var (
				currTime = testCase.StartTime
				mockNow  = func() time.Time {
					return currTime
				}
				mockAfter = func(d time.Duration) <-chan time.Time {
					// calculate it a little differently from the impl here
					expected := testCase.Period - time.Duration(currTime.UnixNano()%testCase.Period.Nanoseconds())*time.Nanosecond
					require.Equal(expected, d, "expected duration to be truncated")

					// time gets updated automatically when After is called
					currTime = currTime.Add(d)

					result := make(chan time.Time, 1)
					result <- currTime
					return result
				}
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ticker := newTruncatedTickerMockable(ctx, testCase.Period, mockNow, mockAfter)

			for _, expectedTick := range testCase.ExpectedTicks {
				select {
				case receivedTick := <-ticker:
					require.Equal(expectedTick, receivedTick)
				case <-testTimeout:
					require.Fail("timed out receiving tick")
				}
			}
		})
	}
}
