package plan

import (
	"github.com/mmlt/environment-operator/pkg/step"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func Test_planFilter(t *testing.T) {
	nsn := types.NamespacedName{
		Namespace: "default",
		Name:      "test",
	}

	planInfraDestroy := plan{
		&step.InfraStep{
			Metaa: stepMeta(nsn, "", step.TypeInfra, ""),
		},
		&step.DestroyStep{
			Metaa: stepMeta(nsn, "", step.TypeDestroy, ""),
		},
	}

	type args struct {
		pl      plan
		allowed map[step.Type]struct{}
	}
	tests := []struct {
		it   string
		args args
		want plan
	}{
		{
			it: "should not remove steps when allowed is nil",
			args: args{
				pl:      planInfraDestroy,
				allowed: nil,
			},
			want: planInfraDestroy,
		},
		{
			it: "should not remove steps when allowed is empty",
			args: args{
				pl:      planInfraDestroy,
				allowed: map[step.Type]struct{}{},
			},
			want: planInfraDestroy,
		},
		{
			it: "should have allowed steps only",
			args: args{
				pl: planInfraDestroy,
				allowed: map[step.Type]struct{}{
					step.TypeDestroy: {},
				},
			},
			want: plan{
				&step.DestroyStep{
					Metaa: stepMeta(nsn, "", step.TypeDestroy, ""),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			got := planFilter(tt.args.pl, tt.args.allowed)
			assert.Equal(t, tt.want, got)
		})
	}
}
