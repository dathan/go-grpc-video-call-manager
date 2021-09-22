package tasks

import (
	"context"
	"reflect"
	"testing"

	"github.com/dathan/go-grpc-video-call-manager/pkg/manager"
)

func Test_server_OpenMeetUrl(t *testing.T) {
	type fields struct {
		UnimplementedOpenMeetUrlServer manager.UnimplementedOpenMeetUrlServer
	}
	type args struct {
		c   context.Context
		man *manager.Meet
	}

	input := args{
		context.Background(),
		&manager.Meet{Uri: "https://meet.google.com/frt-ywwd-epk"},
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *manager.Status
		wantErr bool
	}{
		// TODO: Add test cases.
		{"open", fields{}, input, &manager.Status{Ok: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := server{
				UnimplementedOpenMeetUrlServer: tt.fields.UnimplementedOpenMeetUrlServer,
			}
			got, err := s.OpenMeetUrl(tt.args.c, tt.args.man)
			if (err != nil) != tt.wantErr {
				t.Errorf("server.OpenMeetUrl() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("server.OpenMeetUrl() = %v, want %v", got, tt.want)
			}
		})
	}
}
