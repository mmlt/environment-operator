// Code generated by "stringer -type Type -trimprefix Type"; DO NOT EDIT.

package step

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[TypeInit-0]
	_ = x[TypePlan-1]
	_ = x[TypeApply-2]
	_ = x[TypeAKSPool-3]
	_ = x[TypeKubeconfig-4]
	_ = x[TypeAddons-5]
	_ = x[TypeTest-6]
	_ = x[TypeLast-7]
}

const _Type_name = "InitPlanApplyAKSPoolKubeconfigAddonsTestLast"

var _Type_index = [...]uint8{0, 4, 8, 13, 20, 30, 36, 40, 44}

func (i Type) String() string {
	if i < 0 || i >= Type(len(_Type_index)-1) {
		return "Type(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Type_name[_Type_index[i]:_Type_index[i+1]]
}