module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver

go 1.18

require (
	github.com/census-instrumentation/opencensus-proto v0.4.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.62.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.62.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver v0.62.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver v0.62.0
	github.com/stretchr/testify v1.8.0
	go.opentelemetry.io/collector v0.62.2-0.20221017171445-6313054b642c
	google.golang.org/protobuf v1.28.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf v1.4.3 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.62.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/collector/pdata v0.62.2-0.20221017171445-6313054b642c // indirect
	go.opentelemetry.io/collector/semconv v0.62.2-0.20221017171445-6313054b642c // indirect
	go.opentelemetry.io/otel v1.11.0 // indirect
	go.opentelemetry.io/otel/metric v0.32.3 // indirect
	go.opentelemetry.io/otel/sdk v1.11.0 // indirect
	go.opentelemetry.io/otel/trace v1.11.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.23.0 // indirect
	golang.org/x/net v0.0.0-20220624214902-1bab6f366d9e // indirect
	golang.org/x/sys v0.0.0-20220919091848-fb04ddd9f9c8 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20220822174746-9e6da59bd2fc // indirect
	google.golang.org/grpc v1.50.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver => ../collectdreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver => ../carbonreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../pkg/translator/opencensus

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal
