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

package filtermatcher // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filtermatcher"

import (
	"errors"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterhelper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

type AttributesMatcher []AttributeMatcher

// AttributeMatcher is a attribute key/value pair to match to.
type AttributeMatcher struct {
	Key string
	// If both AttributeValue and StringFilter are nil only check for key existence.
	AttributeValue *pcommon.Value
	// StringFilter is needed to match against a regular expression
	StringFilter filterset.FilterSet
}

var errUnexpectedAttributeType = errors.New("unexpected attribute type")

func NewAttributesMatcher(config filterset.Config, attributes []filterconfig.Attribute) (AttributesMatcher, error) {
	// Convert attribute values from mp representation to in-memory representation.
	var rawAttributes []AttributeMatcher
	for _, attribute := range attributes {

		if attribute.Key == "" {
			return nil, errors.New("can't have empty key in the list of attributes")
		}

		entry := AttributeMatcher{
			Key: attribute.Key,
		}
		if attribute.Value != nil {
			val, err := filterhelper.NewAttributeValueRaw(attribute.Value)
			if err != nil {
				return nil, err
			}

			switch config.MatchType {
			case filterset.Regexp:
				if val.Type() != pcommon.ValueTypeStr {
					return nil, fmt.Errorf(
						"%s=%s for %q only supports Str, but found %s",
						filterset.MatchTypeFieldName, filterset.Regexp, attribute.Key, val.Type(),
					)
				}

				filter, err := filterset.CreateFilterSet([]string{val.Str()}, &config)
				if err != nil {
					return nil, err
				}
				entry.StringFilter = filter
			case filterset.Strict:
				entry.AttributeValue = &val
			default:
				return nil, filterset.NewUnrecognizedMatchTypeError(config.MatchType)

			}
		}

		rawAttributes = append(rawAttributes, entry)
	}
	return rawAttributes, nil
}

// Match attributes specification against a span/log.
func (ma AttributesMatcher) Match(attrs pcommon.Map) bool {
	// If there are no attributes to match against, the span/log matches.
	if len(ma) == 0 {
		return true
	}

	// At this point, it is expected of the span/log to have attributes because of
	// len(ma) != 0. This means for spans/logs with no attributes, it does not match.
	if attrs.Len() == 0 {
		return false
	}

	// Check that all expected properties are set.
	for _, property := range ma {
		attr, exist := attrs.Get(property.Key)
		if !exist {
			return false
		}

		if property.StringFilter != nil {
			value, err := attributeStringValue(attr)
			if err != nil || !property.StringFilter.Matches(value) {
				return false
			}
		} else if property.AttributeValue != nil {
			if !attr.Equal(*property.AttributeValue) {
				return false
			}
		}
	}
	return true
}

func attributeStringValue(attr pcommon.Value) (string, error) {
	switch attr.Type() {
	case pcommon.ValueTypeStr:
		return attr.Str(), nil
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(attr.Bool()), nil
	case pcommon.ValueTypeDouble:
		return strconv.FormatFloat(attr.Double(), 'f', -1, 64), nil
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(attr.Int(), 10), nil
	default:
		return "", errUnexpectedAttributeType
	}
}
