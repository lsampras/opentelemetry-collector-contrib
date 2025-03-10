// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ottlcommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ottlcommon"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ResourceContext interface {
	GetResource() pcommon.Resource
}

func ResourcePathGetSetter[K ResourceContext](path []ottl.Field) (ottl.GetSetter[K], error) {
	if len(path) == 0 {
		return accessResource[K](), nil
	}
	switch path[0].Name {
	case "attributes":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessResourceAttributes[K](), nil
		}
		return accessResourceAttributesKey[K](mapKey), nil
	case "dropped_attributes_count":
		return accessResourceDroppedAttributesCount[K](), nil
	}

	return nil, fmt.Errorf("invalid resource path expression %v", path)
}

func accessResource[K ResourceContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetResource()
		},
		Setter: func(ctx K, val interface{}) {
			if newRes, ok := val.(pcommon.Resource); ok {
				newRes.CopyTo(ctx.GetResource())
			}
		},
	}
}

func accessResourceAttributes[K ResourceContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetResource().Attributes()
		},
		Setter: func(ctx K, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				attrs.CopyTo(ctx.GetResource().Attributes())
			}
		},
	}
}

func accessResourceAttributesKey[K ResourceContext](mapKey *string) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return GetMapValue(ctx.GetResource().Attributes(), *mapKey)
		},
		Setter: func(ctx K, val interface{}) {
			SetMapValue(ctx.GetResource().Attributes(), *mapKey, val)
		},
	}
}

func accessResourceDroppedAttributesCount[K ResourceContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return int64(ctx.GetResource().DroppedAttributesCount())
		},
		Setter: func(ctx K, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetResource().SetDroppedAttributesCount(uint32(i))
			}
		},
	}
}
