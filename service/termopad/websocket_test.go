package termopad

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kirsrus/termopad/server2/model"

	"github.com/juju/errors"
	"github.com/k0kubun/pp"
	"github.com/sirupsen/logrus"
)

func TestNewWebsocket(t *testing.T) {
	type args struct {
		ctx    context.Context
		config *ConfigWebsocket
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "корректный",
			args: args{
				ctx: context.Background(),
				config: &ConfigWebsocket{
					Log: nil,
					TermopadInfo: model.TermopadInfo{
						ID:   1,
						URL:  "ws://192.168.10.10:8000/feed",
						Name: "T1",
					},
					ReconnectTimeout: 0,
					DownloadTimeout:  0,
				},
			},
			wantErr: false,
		},
		{
			name: "не корректный по ID",
			args: args{
				ctx: context.Background(),
				config: &ConfigWebsocket{
					Log: nil,
					TermopadInfo: model.TermopadInfo{
						ID:   0,
						URL:  "ws://192.168.10.10:8000/feed",
						Name: "T1",
					},
					ReconnectTimeout: 0,
					DownloadTimeout:  0,
				},
			},
			wantErr: true,
		}, {
			name: "не корректный по URL",
			args: args{
				ctx: context.Background(),
				config: &ConfigWebsocket{
					Log: nil,
					TermopadInfo: model.TermopadInfo{
						ID:   1,
						URL:  "http://192.168.10.10:8000/feed",
						Name: "T1",
					},
					ReconnectTimeout: 0,
					DownloadTimeout:  0,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ConfigWebsocket{
				TermopadInfo: model.TermopadInfo{
					ID:   tt.args.config.TermopadInfo.ID,
					URL:  tt.args.config.TermopadInfo.URL,
					Name: tt.args.config.TermopadInfo.Name,
				},
			}
			got, err := NewWebsocket(tt.args.ctx, &config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWebsocket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_ = got
		})
	}
}

// TestWebsocket тестирует работу с тестовым термопадом (нужен активный тестер)
func TestWebsocket(t *testing.T) {

	t.Run("простое обращение", func(t *testing.T) {
		testTimeout := 1000 * time.Second
		const logKey = "logger"

		log := logrus.New()
		log.Out = os.Stderr
		log.Level = logrus.DebugLevel
		ctxRoot := context.WithValue(context.Background(), logKey, log)

		ctx, cancel := context.WithCancel(ctxRoot)
		termopadID := uint(1)
		address := "ws://127.0.0.1:8000/feed"
		message := make(chan *model.TermopadTemperatureEvent, 1)

		config := ConfigWebsocket{
			Log: log,
			TermopadInfo: model.TermopadInfo{
				ID:   termopadID,
				URL:  address,
				Name: "T1",
			},
		}

		termopasSvc, err := NewWebsocket(ctx, &config)
		if err != nil {
			t.Fatal(errors.ErrorStack(err))
		}

		// Постоянное получение данных
		go func() {
			for {
				temperature, err := termopasSvc.EmmitTemperature()
				if err != nil {
					if err != ctx.Err() {
						t.Error(errors.ErrorStack(err))
					}
					return
				}
				message <- temperature
			}
		}()

		// Через сколько времени завершать тест
		go func() {
			<-time.After(testTimeout)
			cancel()
		}()

		// Обработка полученных данынх
	endLoop:
		for {
			select {
			case msg := <-message:
				_, _ = pp.Println(msg)
			case <-ctx.Done():
				break endLoop
			}
		}

		time.Sleep(5 * time.Second)

	})

}
