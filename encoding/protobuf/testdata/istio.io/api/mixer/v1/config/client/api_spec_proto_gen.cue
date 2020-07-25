// Copyright 2017 Istio Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.
package client

import "istio.io/api/mixer/v1"

// HTTPAPISpec defines the canonical configuration for generating
// API-related attributes from HTTP requests based on the method and
// uri templated path matches. It is sufficient for defining the API
// surface of a service for the purposes of API attribute
// generation. It is not intended to represent auth, quota,
// documentation, or other information commonly found in other API
// specifications, e.g. OpenAPI.
//
// Existing standards that define operations (or methods) in terms of
// HTTP methods and paths can be normalized to this format for use in
// Istio. For example, a simple petstore API described by OpenAPIv2
// [here](https://github.com/googleapis/gnostic/blob/master/examples/v2.0/yaml/petstore-simple.yaml)
// can be represented with the following HTTPAPISpec.
//
// ```yaml
// apiVersion: config.istio.io/v1alpha2
// kind: HTTPAPISpec
// metadata:
//   name: petstore
//   namespace: default
// spec:
//   attributes:
//     attributes:
//       api.service:
//         stringValue: petstore.swagger.io
//       api.version:
//         stringValue: 1.0.0
//   patterns:
//   - attributes:
//       attributes:
//         api.operation:
//           stringValue: findPets
//     httpMethod: GET
//     uriTemplate: /api/pets
//   - attributes:
//       attributes:
//         api.operation:
//           stringValue: addPet
//     httpMethod: POST
//     uriTemplate: /api/pets
//   - attributes:
//       attributes:
//         api.operation:
//           stringValue: findPetById
//     httpMethod: GET
//     uriTemplate: /api/pets/{id}
//   - attributes:
//       attributes:
//         api.operation:
//           stringValue: deletePet
//     httpMethod: DELETE
//     uriTemplate: /api/pets/{id}
//   api_keys:
//   - query: api-key
// ```
#HTTPAPISpec: {
	// List of attributes that are generated when *any* of the HTTP
	// patterns match. This list typically includes the "api.service"
	// and "api.version" attributes.
	attributes?: v1.#Attributes @protobuf(1,type=Attributes)

	// List of HTTP patterns to match.
	patterns?: [...#HTTPAPISpecPattern] @protobuf(2)

	// List of APIKey that describes how to extract an API-KEY from an
	// HTTP request. The first API-Key match found in the list is used,
	// i.e. 'OR' semantics.
	//
	// The following default policies are used to generate the
	// `request.api_key` attribute if no explicit APIKey is defined.
	//
	//     `query: key, `query: api_key`, and then `header: x-api-key`
	//
	apiKeys?: [...#APIKey] @protobuf(3,name=api_keys)
}

// HTTPAPISpecPattern defines a single pattern to match against
// incoming HTTP requests. The per-pattern list of attributes is
// generated if both the http_method and uri_template match. In
// addition, the top-level list of attributes in the HTTPAPISpec is also
// generated.
//
// ```yaml
// pattern:
// - attributes
//     api.operation: doFooBar
//   httpMethod: GET
//   uriTemplate: /foo/bar
// ```
#HTTPAPISpecPattern: {
	// List of attributes that are generated if the HTTP request matches
	// the specified http_method and uri_template. This typically
	// includes the "api.operation" attribute.
	attributes?: v1.#Attributes @protobuf(1,type=Attributes)

	// HTTP request method to match against as defined by
	// [rfc7231](https://tools.ietf.org/html/rfc7231#page-21). For
	// example: GET, HEAD, POST, PUT, DELETE.
	httpMethod?: string @protobuf(2,name=http_method)
	{} | {
		// URI template to match against as defined by
		// [rfc6570](https://tools.ietf.org/html/rfc6570). For example, the
		// following are valid URI templates:
		//
		//     /pets
		//     /pets/{id}
		//     /dictionary/{term:1}/{term}
		//     /search{?q*,lang}
		//
		uriTemplate: string @protobuf(3,name=uri_template)
	} | {
		// EXPERIMENTAL:
		//
		// ecmascript style regex-based match as defined by
		// [EDCA-262](http://en.cppreference.com/w/cpp/regex/ecmascript). For
		// example,
		//
		//     "^/pets/(.*?)?"
		//
		regex: string @protobuf(4)
	}
}

// APIKey defines the explicit configuration for generating the
// `request.api_key` attribute from HTTP requests.
//
// See [API Keys](https://swagger.io/docs/specification/authentication/api-keys)
// for a general overview of API keys as defined by OpenAPI.
#APIKey: {
	{} | {
		// API Key is sent as a query parameter. `query` represents the
		// query string parameter name.
		//
		// For example, `query=api_key` should be used with the
		// following request:
		//
		//     GET /something?api_key=abcdef12345
		//
		query: string @protobuf(1)
	} | {
		// API key is sent in a request header. `header` represents the
		// header name.
		//
		// For example, `header=X-API-KEY` should be used with the
		// following request:
		//
		//     GET /something HTTP/1.1
		//     X-API-Key: abcdef12345
		//
		header: string @protobuf(2)
	} | {
		// API key is sent in a
		// [cookie](https://swagger.io/docs/specification/authentication/cookie-authentication),
		//
		// For example, `cookie=X-API-KEY` should be used for the
		// following request:
		//
		//     GET /something HTTP/1.1
		//     Cookie: X-API-KEY=abcdef12345
		//
		cookie: string @protobuf(3)
	}
}

// HTTPAPISpecReference defines a reference to an HTTPAPISpec. This is
// typically used for establishing bindings between an HTTPAPISpec and an
// IstioService. For example, the following defines an
// HTTPAPISpecReference for service `foo` in namespace `bar`.
//
// ```yaml
// - name: foo
//   namespace: bar
// ```
#HTTPAPISpecReference: {
	// REQUIRED. The short name of the HTTPAPISpec. This is the resource
	// name defined by the metadata name field.
	name?: string @protobuf(1)

	// Optional namespace of the HTTPAPISpec. Defaults to the encompassing
	// HTTPAPISpecBinding's metadata namespace field.
	namespace?: string @protobuf(2)
}

// HTTPAPISpecBinding defines the binding between HTTPAPISpecs and one or more
// IstioService. For example, the following establishes a binding
// between the HTTPAPISpec `petstore` and service `foo` in namespace `bar`.
//
// ```yaml
// apiVersion: config.istio.io/v1alpha2
// kind: HTTPAPISpecBinding
// metadata:
//   name: my-binding
//   namespace: default
// spec:
//   services:
//   - name: foo
//     namespace: bar
//   api_specs:
//   - name: petstore
//     namespace: default
// ```
#HTTPAPISpecBinding: {
	// REQUIRED. One or more services to map the listed HTTPAPISpec onto.
	services?: [...#IstioService] @protobuf(1)

	// REQUIRED. One or more HTTPAPISpec references that should be mapped to
	// the specified service(s). The aggregate collection of match
	// conditions defined in the HTTPAPISpecs should not overlap.
	apiSpecs?: [...#HTTPAPISpecReference] @protobuf(2,name=api_specs)
}
