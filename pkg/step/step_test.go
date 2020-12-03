package step

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTypesFromString(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		it   string
		args args
		want map[Type]struct{}
		err  error
	}{
		{
			it:   "should return an empty set on empty input",
			args: args{s: ""},
			want: map[Type]struct{}{},
			err:  nil,
		},
		{
			it:   "should return an set with types for valid input",
			args: args{s: "Infra,Destroy"},
			want: map[Type]struct{}{
				TypeInfra:   struct{}{},
				TypeDestroy: struct{}{},
			},
			err: nil,
		},
		{
			it:   "should return an error for unknown step types",
			args: args{s: "Infra,XXXX"},
			want: map[Type]struct{}{
				TypeInfra: struct{}{},
			},
			err: errors.New("unknown step type(s): [XXXX]"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			got, err := TypesFromString(tt.args.s)
			assert.Equal(t, tt.err, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
