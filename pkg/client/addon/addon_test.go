package addon

import (
	"github.com/mmlt/testr"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

func Test_parseAddonResponseLine(t *testing.T) {
	tsts := []struct {
		it   string
		in   []string
		want []KTResult
	}{
		{
			it: "must handle happy input",
			in: []string{
				"I 18:24:53  \"level\"=0 \"txt\"=\"namespace/kube-system unchanged\" \"msg\"=\"apply\" \"id\"=\"1\"  \"tpl\"=\"namespace.yaml\"\n",
				"I 18:24:54  \"level\"=0 \"txt\"=\"namespace/xyz-system created\" \"msg\"=\"apply\"  \"id\"=\"2\" \"tpl\"=\"namespace.yaml\"\n",
				"I 18:24:55  \"level\"=0 \"txt\"=\"pod/opa-5cd59b58bc-rrrxf condition met\" \"msg\"=\"wait\"  \"id\"=\"3\"  \"tpl\"=\"\"\n",
			},
			want: []KTResult{
				{Added: 0, Changed: 0, Deleted: 0, Errors: []string(nil), Object: "namespace/kube-system unchanged", ObjectID: "1", Action: "apply"},
				{Added: 0, Changed: 0, Deleted: 0, Errors: []string(nil), Object: "namespace/xyz-system created", ObjectID: "2", Action: "apply"},
				{Added: 0, Changed: 0, Deleted: 0, Errors: []string(nil), Object: "pod/opa-5cd59b58bc-rrrxf condition met", ObjectID: "3", Action: "wait"},
			},
		},
		{
			it: "must handle dotted id",
			in: []string{
				"I 18:24:53  \"level\"=0 \"txt\"=\"namespace/kube-system unchanged\" \"msg\"=\"apply\" \"id\"=\"01.02\"  \"tpl\"=\"namespace.yaml\"\n",
			},
			want: []KTResult{
				{Added: 0, Changed: 0, Deleted: 0, Errors: []string(nil), Object: "namespace/kube-system unchanged", ObjectID: "01.02", Action: "apply"},
			},
		},
		{
			it: "must handle all input",
			in: []string{
				"I 09:14:40  \"level\"=0 \"msg\"=\"apply\"  \"id\"=\"25\" \"tpl\"=\"hashi-injector.yaml\" \"txt\"=\"service/vault-agent-injector-svc created\"\n",
			},
			want: []KTResult{
				{Added: 0, Changed: 0, Deleted: 0, Errors: []string(nil), Object: "service/vault-agent-injector-svc created", ObjectID: "25", Action: "apply"},
			},
		},
		{
			it: "must handle template errors",
			in: []string{
				"I 18:24:54  \"level\"=0 \"msg\"=\"apply\"  \"id\"=\"1\" \"txt\"=\"namespace/kube-system unchanged\" \"tpl\"=\"namespace.yaml\"\n",
				"E expand ../../../tpl/cert-manager/cert-manager.yaml: execute: template: input:5944:7: executing \"input\" at <eq .Values.k8sProvider \"minikube\">: error calling eq: incompatible types for comparison\n",
			},
			want: []KTResult{
				{Added: 0, Changed: 0, Deleted: 0, Errors: []string(nil), Object: "namespace/kube-system unchanged", ObjectID: "1", Action: "apply"},
				{Added: 0, Changed: 0, Deleted: 0, Errors: []string{"expand ../../../tpl/cert-manager/cert-manager.yaml: execute: template: input:5944:7: executing \"input\" at <eq .Values.k8sProvider \"minikube\">: error calling eq: incompatible types for comparison"}, Object: "", ObjectID: "", Action: ""},
			},
		},
		{
			it: "must handle cli errors",
			in: []string{
				"E -job-file should be defined, -m should be one of 'generate' or 'apply'\n",
			},
			want: []KTResult{
				{Added: 0, Changed: 0, Deleted: 0, Errors: []string{"-job-file should be defined, -m should be one of 'generate' or 'apply'"}, Object: "", ObjectID: "", Action: ""},
			},
		},
		{
			it:   "must handle empty input",
			in:   []string{},
			want: []KTResult{},
		},
	}

	ao := &Addon{
		Log: testr.New(t),
	}

	for _, tst := range tsts {
		t.Run(tst.it, func(t *testing.T) {
			rd, wr := io.Pipe()

			// start parser
			ch := ao.parseAsyncAddonResponse(rd)

			// send input
			go func() {
				for _, s := range tst.in {
					wr.Write([]byte(s))
				}
				wr.Close()
			}()

			// read output
			rs := []KTResult{}
			for r := range ch {
				rs = append(rs, r)
			}

			assert.Equal(t, tst.want, rs, "It %s.", tst.it)
		})
	}
}
