package reefd

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type fakeEC2Instance struct {
	id         string
	stateCode  string
	launchTime time.Time
	tags       map[string]string
}

func matchPattern(s, pattern string) bool {
	// only supports leading '*' and tailing '*'
	if pattern == "*" {
		return true
	}
	starStart := strings.HasPrefix(pattern, "*")
	starEnd := strings.HasSuffix(pattern, "*")
	if starStart && starEnd {
		return strings.Contains(s, pattern[1:len(pattern)-1])
	}
	if starStart {
		return strings.HasSuffix(s, pattern[1:])
	}
	if starEnd {
		return strings.HasPrefix(s, pattern[:len(pattern)-1])
	}
	return s == pattern
}

func matchPatterns(s string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(s, pattern) {
			return true
		}
	}
	return false
}

func TestMatchPatterns(t *testing.T) {
	tests := []struct {
		s        string
		patterns []string
		want     bool
	}{
		{"", []string{}, false},
		{"foo", []string{"*"}, true},
		{"foo", []string{"*o"}, true},
		{"foo", []string{"*foo"}, true},
		{"foo", []string{"f*"}, true},
		{"foo", []string{"foo*"}, true},
		{"window", []string{"*window*"}, true},
		{"bk-window", []string{"*window*"}, true},
		{"window-pr", []string{"*window*"}, true},
		{"bk-window-pr", []string{"*window*"}, true},
		{"bk-linux-pr", []string{"*window*"}, false},
		{"linux", []string{"*windows*"}, false},
		{"16", []string{"16", "20"}, true},
		{"16", []string{"20"}, false},
		{"16", []string{"16"}, true},
		{"16", []string{"6"}, false},
	}
	for _, test := range tests {
		got := matchPatterns(test.s, test.patterns)
		if got != test.want {
			t.Errorf(
				"matchPatterns(%q, %q) = %v; want %v",
				test.s, test.patterns, got, test.want,
			)
		}
	}
}

func (i *fakeEC2Instance) matchTag(k string, patterns []string) bool {
	v, ok := i.tags[k]
	if !ok {
		return false
	}
	return matchPatterns(v, patterns)
}

type fakeEC2 struct {
	instances map[string]*fakeEC2Instance
}

func newFakeEC2() *fakeEC2 {
	return &fakeEC2{instances: make(map[string]*fakeEC2Instance)}
}

func (e *fakeEC2) add(id, queue, state string, launchTime time.Time) {
	e.instances[id] = &fakeEC2Instance{
		id:         id,
		stateCode:  state,
		launchTime: launchTime,
		tags: map[string]string{
			"BuildkiteQueue": queue,
		},
	}
}

func (e *fakeEC2) remove(id string) { delete(e.instances, id) }

func (e *fakeEC2) instance(id string) *fakeEC2Instance {
	return e.instances[id]
}

func (e *fakeEC2) ids() []string {
	ids := make([]string, 0, len(e.instances))
	for id := range e.instances {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func instanceMatchFilters(i *fakeEC2Instance, filters []ec2types.Filter) bool {
	for _, filter := range filters {
		// Needs to match all filters.
		if *filter.Name == "instance-state-code" {
			if !matchPatterns(i.stateCode, filter.Values) {
				return false
			}
		} else if strings.HasPrefix(*filter.Name, "tag:") {
			tagName := strings.TrimPrefix(*filter.Name, "tag:")
			if !i.matchTag(tagName, filter.Values) {
				return false
			}
		}
	}
	return true
}

func (e *fakeEC2) DescribeInstances(
	ctx context.Context, in *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	out := &ec2.DescribeInstancesOutput{}
	for _, i := range e.instances {
		if instanceMatchFilters(i, in.Filters) {
			out.Reservations = append(out.Reservations, ec2types.Reservation{
				Instances: []ec2types.Instance{{
					InstanceId: aws.String(i.id),
					LaunchTime: aws.Time(i.launchTime),
				}},
			})
		}
	}
	return out, nil
}

func (e *fakeEC2) TerminateInstances(
	ctx context.Context, in *ec2.TerminateInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.TerminateInstancesOutput, error) {
	out := &ec2.TerminateInstancesOutput{}
	for _, id := range in.InstanceIds {
		if i := e.instance(id); i != nil {
			out.TerminatingInstances = append(
				out.TerminatingInstances,
				ec2types.InstanceStateChange{
					InstanceId: aws.String(id),
					CurrentState: &ec2types.InstanceState{
						Name: ec2types.InstanceStateNameShuttingDown,
					},
				},
			)
		}
	}

	// remove instances
	for _, id := range in.InstanceIds {
		e.remove(id)
	}

	return out, nil
}

func TestListDeadWindowsInstances(t *testing.T) {
	now := time.Now()

	// Setup
	ec2 := newFakeEC2()
	ec2.add("i-w1", "bk-windows-pr", "0", now.Add(-8*time.Hour))
	ec2.add("i-w2", "bk-windows-branch", "16", now.Add(-8*time.Hour))
	ec2.add("i-w3", "bk-windows-branch", "48", now.Add(-8*time.Hour))
	ec2.add("i-w4", "bk-windows-pr", "0", now.Add(-3*time.Hour))
	ec2.add("i-l1", "linux", "0", now.Add(-8*time.Hour))

	r := newReaper(ec2)
	r.setNowFunc(func() time.Time { return now })

	ctx := context.Background()
	ids, err := r.listDeadWindowsInstances(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify
	want := []string{"i-w1", "i-w2"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
}

func TestTerminateInsta(t *testing.T) {
	now := time.Now()

	ec2 := newFakeEC2()
	ec2.add("i-w1", "bk-windows-pr", "0", now.Add(-8*time.Hour))
	ec2.add("i-w2", "bk-windows-branch", "16", now.Add(-8*time.Hour))

	r := newReaper(ec2)
	ctx := context.Background()
	if err := r.terminateInstances(ctx, []string{"i-w1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, want := ec2.ids(), []string{"i-w2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got instances %v, want %v", got, want)
	}

	if err := r.terminateInstances(ctx, []string{"i-na"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, want = ec2.ids(), []string{"i-w2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got instances %v, want %v", got, want)
	}

	if err := r.terminateInstances(ctx, []string{"i-w2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, want = ec2.ids(), []string{}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got instances %v, want %v", got, want)
	}
}

func TestListAndReapDeadWindowsInstances(t *testing.T) {
	now := time.Now()

	// Setup
	ec2 := newFakeEC2()
	ec2.add("i-w1", "bk-windows-pr", "0", now.Add(-8*time.Hour))
	ec2.add("i-w2", "bk-windows-branch", "16", now.Add(-8*time.Hour))
	ec2.add("i-w3", "bk-windows-branch", "48", now.Add(-8*time.Hour))
	ec2.add("i-w4", "bk-windows-pr", "0", now.Add(-3*time.Hour))
	ec2.add("i-l1", "linux", "0", now.Add(-8*time.Hour))

	r := newReaper(ec2)
	r.setNowFunc(func() time.Time { return now })

	ctx := context.Background()
	n, err := r.listAndReapDeadWindowsInstances(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Fatalf("got %d instance reaped, want %d", n, 2)
	}

	// Verify
	ids := ec2.ids()
	wantIDs := []string{"i-l1", "i-w3", "i-w4"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("got %v left, want %v", ids, wantIDs)
	}

	// Reap again.
	n, err = r.listAndReapDeadWindowsInstances(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("got %d instance reaped the 2nd time, want none", n)
	}

	// Verify again.
	ids = ec2.ids()
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("got %v left, want %v", ids, wantIDs)
	}
}

// errorEC2 is a fake EC2 client that returns configurable errors
type errorEC2 struct {
	describeErr  error
	terminateErr error
}

func (e *errorEC2) DescribeInstances(
	ctx context.Context, in *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	if e.describeErr != nil {
		return nil, e.describeErr
	}
	return &ec2.DescribeInstancesOutput{}, nil
}

func (e *errorEC2) TerminateInstances(
	ctx context.Context, in *ec2.TerminateInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.TerminateInstancesOutput, error) {
	return nil, e.terminateErr
}

func TestListDeadWindowsInstancesError(t *testing.T) {
	wantErr := errors.New("AWS API error")
	r := newReaper(&errorEC2{describeErr: wantErr})

	ctx := context.Background()
	_, err := r.listDeadWindowsInstances(ctx)
	if err == nil {
		t.Fatal("listDeadWindowsInstances() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "describe instances") {
		t.Errorf("listDeadWindowsInstances() error = %q, want error containing 'describe instances'", err)
	}
}

func TestTerminateInstancesError(t *testing.T) {
	wantErr := errors.New("terminate failed")
	r := newReaper(&errorEC2{terminateErr: wantErr})

	ctx := context.Background()
	err := r.terminateInstances(ctx, []string{"i-123"})
	if err == nil {
		t.Fatal("terminateInstances() error = nil, want error")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("terminateInstances() error = %v, want %v", err, wantErr)
	}
}

func TestTerminateInstancesEmpty(t *testing.T) {
	r := newReaper(&errorEC2{terminateErr: errors.New("should not be called")})

	ctx := context.Background()
	err := r.terminateInstances(ctx, []string{})
	if err != nil {
		t.Errorf("terminateInstances() with empty ids error = %v, want nil", err)
	}

	err = r.terminateInstances(ctx, nil)
	if err != nil {
		t.Errorf("terminateInstances() with nil ids error = %v, want nil", err)
	}
}

// terminateErrorEC2 returns instances to describe but fails on terminate
type terminateErrorEC2 struct {
	instances    []*fakeEC2Instance
	terminateErr error
}

func (e *terminateErrorEC2) DescribeInstances(
	ctx context.Context, in *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	out := &ec2.DescribeInstancesOutput{}
	for _, i := range e.instances {
		if instanceMatchFilters(i, in.Filters) {
			out.Reservations = append(out.Reservations, ec2types.Reservation{
				Instances: []ec2types.Instance{{
					InstanceId: aws.String(i.id),
					LaunchTime: aws.Time(i.launchTime),
				}},
			})
		}
	}
	return out, nil
}

func (e *terminateErrorEC2) TerminateInstances(
	ctx context.Context, in *ec2.TerminateInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.TerminateInstancesOutput, error) {
	return nil, e.terminateErr
}

func TestListAndReapDeadWindowsInstancesTerminateError(t *testing.T) {
	now := time.Now()
	wantErr := errors.New("terminate failed")

	ec2Client := &terminateErrorEC2{
		instances: []*fakeEC2Instance{{
			id:         "i-w1",
			stateCode:  "16",
			launchTime: now.Add(-8 * time.Hour),
			tags:       map[string]string{"BuildkiteQueue": "bk-windows-pr"},
		}},
		terminateErr: wantErr,
	}

	r := newReaper(ec2Client)
	r.setNowFunc(func() time.Time { return now })

	ctx := context.Background()
	n, err := r.listAndReapDeadWindowsInstances(ctx)
	if err == nil {
		t.Fatal("listAndReapDeadWindowsInstances() error = nil, want error")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("listAndReapDeadWindowsInstances() error = %v, want %v", err, wantErr)
	}
	if n != 0 {
		t.Errorf("listAndReapDeadWindowsInstances() n = %d, want 0 on error", n)
	}
}

func TestListAndReapDeadWindowsInstancesDescribeError(t *testing.T) {
	wantErr := errors.New("describe failed")
	r := newReaper(&errorEC2{describeErr: wantErr})

	ctx := context.Background()
	n, err := r.listAndReapDeadWindowsInstances(ctx)
	if err == nil {
		t.Fatal("listAndReapDeadWindowsInstances() error = nil, want error")
	}
	if n != 0 {
		t.Errorf("listAndReapDeadWindowsInstances() n = %d, want 0 on error", n)
	}
}

func TestReaperNowDefault(t *testing.T) {
	r := newReaper(newFakeEC2())
	// nowFunc is nil by default, so now() should return current time
	before := time.Now()
	got := r.now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("now() = %v, want time between %v and %v", got, before, after)
	}
}
