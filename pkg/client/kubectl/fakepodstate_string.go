// Code generated by "stringer -type=FakePodState"; DO NOT EDIT.

package kubectl

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[FakePodUnknown-0]
	_ = x[FakePodRunning-1]
	_ = x[FakePodCompleted-2]
	_ = x[FakePodError-3]
}

const _FakePodState_name = "FakePodUnknownFakePodRunningFakePodCompletedFakePodError"

var _FakePodState_index = [...]uint8{0, 14, 28, 44, 56}

func (i FakePodState) String() string {
	if i < 0 || i >= FakePodState(len(_FakePodState_index)-1) {
		return "FakePodState(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _FakePodState_name[_FakePodState_index[i]:_FakePodState_index[i+1]]
}
