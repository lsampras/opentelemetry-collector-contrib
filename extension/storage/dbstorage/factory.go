// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
)

// The value of extension "type" in configuration.
const typeStr config.Type = "db_storage"

// NewFactory creates a factory for DBStorage extension.
func NewFactory() component.ExtensionFactory {
	return component.NewExtensionFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelAlpha,
	)
}

func createDefaultConfig() config.Extension {
	return &Config{
		ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
	}
}

func createExtension(
	_ context.Context,
	params component.ExtensionCreateSettings,
	cfg config.Extension,
) (component.Extension, error) {
	return newDBStorage(params.Logger, cfg.(*Config))
}
