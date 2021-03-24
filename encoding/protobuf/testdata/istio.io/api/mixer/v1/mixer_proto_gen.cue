// Copyright 2016 Istio Authors
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

// This package defines the Mixer API that the sidecar proxy uses to perform
// precondition checks, manage quotas, and report telemetry.
package v1

import (
	status_1 "google.golang.org/genproto/googleapis/rpc/status"
	"time"
)

// Used to get a thumbs-up/thumbs-down before performing an action.
#CheckRequest: {
	// parameters for a quota allocation
	#QuotaParams: {
		// Amount of quota to allocate
		amount?: int64 @protobuf(1,int64)

		// When true, supports returning less quota than what was requested.
		bestEffort?: bool @protobuf(2,bool,name=best_effort)
	}

	// The attributes to use for this request.
	//
	// Mixer's configuration determines how these attributes are used to
	// establish the result returned in the response.
	attributes?: #CompressedAttributes @protobuf(1,CompressedAttributes,"(gogoproto.nullable)=false")

	// The number of words in the global dictionary, used with to populate the attributes.
	// This value is used as a quick way to determine whether the client is using a dictionary that
	// the server understands.
	globalWordCount?: uint32 @protobuf(2,uint32,name=global_word_count)

	// Used for deduplicating `Check` calls in the case of failed RPCs and retries. This should be a UUID
	// per call, where the same UUID is used for retries of the same call.
	deduplicationId?: string @protobuf(3,string,name=deduplication_id)

	// The individual quotas to allocate
	quotas?: {
		[string]: #QuotaParams
	} @protobuf(4,map[string]QuotaParams,"(gogoproto.nullable)=false")
}

// The response generated by the Check method.
#CheckResponse: {
	// Expresses the result of a precondition check.
	#PreconditionResult: {
		// A status code of OK indicates all preconditions were satisfied. Any other code indicates not
		// all preconditions were satisfied and details describe why.
		status?: status_1.#Status @protobuf(1,google.rpc.Status,"(gogoproto.nullable)=false")

		// The amount of time for which this result can be considered valid.
		validDuration?: time.Duration @protobuf(2,google.protobuf.Duration,name=valid_duration,"(gogoproto.nullable)=false","(gogoproto.stdduration)")

		// The number of uses for which this result can be considered valid.
		validUseCount?: int32 @protobuf(3,int32,name=valid_use_count)

		// The total set of attributes that were used in producing the result
		// along with matching conditions.
		referencedAttributes?: #ReferencedAttributes @protobuf(5,ReferencedAttributes,name=referenced_attributes)

		// An optional routing directive, used to manipulate the traffic metadata
		// whenever all preconditions are satisfied.
		routeDirective?: #RouteDirective @protobuf(6,RouteDirective,name=route_directive)
	}

	// Expresses the result of a quota allocation.
	#QuotaResult: {
		// The amount of time for which this result can be considered valid.
		validDuration?: time.Duration @protobuf(1,google.protobuf.Duration,name=valid_duration,"(gogoproto.nullable)=false","(gogoproto.stdduration)")

		// The amount of granted quota. When `QuotaParams.best_effort` is true, this will be >= 0.
		// If `QuotaParams.best_effort` is false, this will be either 0 or >= `QuotaParams.amount`.
		grantedAmount?: int64 @protobuf(2,int64,name=granted_amount)

		// The total set of attributes that were used in producing the result
		// along with matching conditions.
		referencedAttributes?: #ReferencedAttributes @protobuf(5,ReferencedAttributes,name=referenced_attributes,"(gogoproto.nullable)=false")
	}

	// The precondition check results.
	precondition?: #PreconditionResult @protobuf(2,PreconditionResult,"(gogoproto.nullable)=false")

	// The resulting quota, one entry per requested quota.
	quotas?: {
		[string]: #QuotaResult
	} @protobuf(3,map[string]QuotaResult,"(gogoproto.nullable)=false")
}

// Describes the attributes that were used to determine the response.
// This can be used to construct a response cache.
#ReferencedAttributes: {
	// How an attribute's value was matched
	#Condition: "CONDITION_UNSPECIFIED" | // should not occur
		"ABSENCE" | // match when attribute doesn't exist
		"EXACT" | // match when attribute value is an exact byte-for-byte match
		"REGEX" // match when attribute value matches the included regex

	#Condition_value: {
		"CONDITION_UNSPECIFIED": 0
		"ABSENCE":               1
		"EXACT":                 2
		"REGEX":                 3
	}

	// Describes a single attribute match.
	#AttributeMatch: {
		// The name of the attribute. This is a dictionary index encoded in a manner identical
		// to all strings in the [CompressedAttributes][istio.mixer.v1.CompressedAttributes] message.
		name?: int32 @protobuf(1,sint32)

		// The kind of match against the attribute value.
		condition?: #Condition @protobuf(2,Condition)

		// If a REGEX condition is provided for a STRING_MAP attribute,
		// clients should use the regex value to match against map keys.
		regex?: string @protobuf(3,string)

		// A key in a STRING_MAP. When multiple keys from a STRING_MAP
		// attribute were referenced, there will be multiple AttributeMatch
		// messages with different map_key values. Values for map_key SHOULD
		// be ignored for attributes that are not STRING_MAP.
		//
		// Indices for the keys are used (taken either from the
		// message dictionary from the `words` field or the global dictionary).
		//
		// If no map_key value is provided for a STRING_MAP attribute, the
		// entire STRING_MAP will be used.
		mapKey?: int32 @protobuf(4,sint32,name=map_key)
	}

	// The message-level dictionary. Refer to [CompressedAttributes][istio.mixer.v1.CompressedAttributes] for information
	// on using dictionaries.
	words?: [...string] @protobuf(1,string)

	// Describes a set of attributes.
	attributeMatches?: [...#AttributeMatch] @protobuf(2,AttributeMatch,name=attribute_matches,"(gogoproto.nullable)=false")
}

// Operation on HTTP headers to replace, append, or remove a header. Header
// names are normalized to lower-case with dashes, e.g.  "x-request-id".
// Pseudo-headers ":path", ":authority", and ":method" are supported to modify
// the request headers.
#HeaderOperation: {
	// Operation type.
	#Operation: "REPLACE" | // replaces the header with the given name
		"REMOVE" | // removes the header with the given name (the value is ignored)
		"APPEND" // appends the value to the header value, or sets it if not present

	#Operation_value: {
		"REPLACE": 0
		"REMOVE":  1
		"APPEND":  2
	}

	// Header name.
	name?: string @protobuf(1,string)

	// Header value.
	value?: string @protobuf(2,string)

	// Header operation.
	operation?: #Operation @protobuf(3,Operation)
}

// Expresses the routing manipulation actions to be performed on behalf of
// Mixer in response to a precondition check.
#RouteDirective: {
	// Operations on the request headers.
	requestHeaderOperations?: [...#HeaderOperation] @protobuf(1,HeaderOperation,name=request_header_operations,"(gogoproto.nullable)=false")

	// Operations on the response headers.
	responseHeaderOperations?: [...#HeaderOperation] @protobuf(2,HeaderOperation,name=response_header_operations,"(gogoproto.nullable)=false")

	// If set, enables a direct response without proxying the request to the routing
	// destination. Required to be a value in the 2xx or 3xx range.
	directResponseCode?: uint32 @protobuf(3,uint32,name=direct_response_code)

	// Supplies the response body for the direct response.
	// If this setting is omitted, no body is included in the generated response.
	directResponseBody?: string @protobuf(4,string,name=direct_response_body)
}

// Used to report telemetry after performing one or more actions.
#ReportRequest: {
	// next value: 5

	// Used to signal how the sets of compressed attributes should be reconstitued server-side.
	#RepeatedAttributesSemantics:
		// Use delta encoding between sets of compressed attributes to reduce the overall on-wire
		// request size. Each individual set of attributes is used to modify the previous set.
		// NOTE: There is no way with this encoding to specify attribute value deletion. This
		// option should be used with extreme caution.
		"DELTA_ENCODING" |

		// Treat each set of compressed attributes as complete - independent from other sets
		// in this request. This will result in on-wire duplication of attributes and values, but
		// will allow for proper accounting of absent values in overall encoding.
		"INDEPENDENT_ENCODING"

	#RepeatedAttributesSemantics_value: {
		"DELTA_ENCODING":       0
		"INDEPENDENT_ENCODING": 1
	}

	// The attributes to use for this request.
	//
	// Each `Attributes` element represents the state of a single action. Multiple actions
	// can be provided in a single message in order to improve communication efficiency. The
	// client can accumulate a set of actions and send them all in one single message.
	attributes?: [...#CompressedAttributes] @protobuf(1,CompressedAttributes,"(gogoproto.nullable)=false")

	// Indicates how to decode the attributes sets in this request.
	repeatedAttributesSemantics?: #RepeatedAttributesSemantics @protobuf(4,RepeatedAttributesSemantics,name=repeated_attributes_semantics)

	// The default message-level dictionary for all the attributes.
	// Individual attribute messages can have their own dictionaries, but if they don't
	// then this set of words, if it is provided, is used instead.
	//
	// This makes it possible to share the same dictionary for all attributes in this
	// request, which can substantially reduce the overall request size.
	defaultWords?: [...string] @protobuf(2,string,name=default_words)

	// The number of words in the global dictionary.
	// To detect global dictionary out of sync between client and server.
	globalWordCount?: uint32 @protobuf(3,uint32,name=global_word_count)
}

// Used to carry responses to telemetry reports
#ReportResponse: {
}
