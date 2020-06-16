package ci

import (
	"github.com/SchemaStore/schemastore/src/schemas/json"
	encjson "encoding/json"
)

workflowsDir: *"./" | string @tag(workflowsDir)

workflows: [...{file: string, schema: json.#Workflow}]
workflows: [
	{
		file:   "test.yml"
		schema: test
	},
	{
		file:   "test_dispatch.yml"
		schema: test_dispatch
	},
	{
		file:   "release.yml"
		schema: release
	},
	{
		file:   "rebuild_tip_cuelang_org.yml"
		schema: rebuild_tip_cuelang_org
	},
]

// TODO: drop when cuelang.org/issue/390 is fixed.
// Declare definitions for sub-schemas
#job:  (json.#Workflow.jobs & {x: _}).x
#step: ((#job & {steps:           _}).steps & [_])[0]

#latestGo: "1.14.3"

#testStrategy: {
	"fail-fast": false
	matrix: {
		// Use a stable version of 1.14.x for go generate
		"go-version": ["1.12.x", "1.13.x", #latestGo]
		os: ["ubuntu-latest", "macos-latest", "windows-latest"]
	}
}

#installGo: #step & {
	name: "Install Go"
	uses: "actions/setup-go@v2"
	with: "go-version": "${{ matrix.go-version }}"
}

#checkoutCode: #step & {
	name: "Checkout code"
	uses: "actions/checkout@v2"
}

#cacheGoModules: #step & {
	name: "Cache Go modules"
	uses: "actions/cache@v1"
	with: {
		path: "~/go/pkg/mod"
		key:  "${{ runner.os }}-${{ matrix.go-version }}-go-${{ hashFiles('**/go.sum') }}"
		"restore-keys": """
					${{ runner.os }}-${{ matrix.go-version }}-go-
					"""
	}

}

#goGenerate: #step & {
	name: "Generate"
	run:  "go generate ./..."
	// The Go version corresponds to the precise 1.14.x version specified in
	// the matrix. Skip windows for now until we work out why re-gen is flaky
	if: "matrix.go-version == '\(#latestGo)' && matrix.os != 'windows-latest'"
}

#goTest: #step & {
	name: "Test"
	run:  "go test ./..."
}

#goTestRace: #step & {
	name: "Test with -race"
	run:  "go test -race ./..."
}

#goReleaseCheck: #step & {
	name: "gorelease check"
	run:  "go run golang.org/x/exp/cmd/gorelease"
	// Only run on 1.13.x and latest Go for now. Bug with Go 1.12.x means
	// this check fails
	if: "matrix.go-version == '\(#latestGo)' || matrix.go-version == '1.13.x'"
}

#checkGitClean: #step & {
	name: "Check that git is clean post generate and tests"
	run:  "test -z \"$(git status --porcelain)\" || (git status; git diff; false)"
}

#pullThroughProxy: #step & {
	name: "Pull this commit through the proxy on master"
	run: """
				v=$(git rev-parse HEAD)
				cd $(mktemp -d)
				go mod init mod.com
				GOPROXY=https://proxy.golang.org go get -d cuelang.org/go@$v
				"""
	if: "github.ref == 'refs/heads/master'"
}

test: json.#Workflow & {
	#setCUEEnv: #step & {
		name: "Set build branch env vars"
		run: #"""
			echo "::set-env name=CUE_IS_BUILD_BRANCH::$(echo $GITHUB_REF | grep -q '^refs\/heads\/[[:digit:]]\{14\}-I[0-9a-f]\+-[0-9a-f]\+-refs_changes_[[:digit:]]\{2\}_[[:digit:]]\+_[[:digit:]]\+$' && echo true || echo false)"
			echo "::set-env name=CUE_CHANGE_ID::$(echo $GITHUB_REF | cut -d '-' -f 2)"
			echo "::set-env name=CUE_COMMIT::$(echo $GITHUB_REF | cut -d '-' -f 3)"
			echo "::set-env name=CUE_REF::$(echo $GITHUB_REF | cut -d '-' -f 4 | sed 's/_/\//g')"
			"""#
	}
	#writeCookiesFile: #step & {
		name: "Write the gitcookies file"
		run:  "echo \"$GERRIT_COOKIE\" > ~/.gitcookies"
	}
	#gerrit: {
		#setCodeReview: {
			#args: {
				message: string
				labels?: {
					"Code-Review": int
				}
			}
			res: #"""
			curl -f -s -H "Content-Type: application/json" --request POST --data '\#(encjson.Marshal(#args))' -b ~/.gitcookies https://cue-review.googlesource.com/a/changes/$CUE_CHANGE_ID/revisions/$CUE_COMMIT/review
			"""#
		}
	}

	env: GERRIT_COOKIE: "${{ secrets.gerritCookie }}"
	name: "Test"
	on: {
		push: {
			branches: ["*"]
			"tags-ignore": ["v*"]
		}
	}
	defaults: run: shell: "bash"
	jobs: {
		start: {
			"runs-on": "ubuntu-latest"
			steps: [
				#setCUEEnv,
				#writeCookiesFile,
				#step & {
					if:   "${{ env.CUE_IS_BUILD_BRANCH == 'true' }}"
					name: "Update Gerrit CL message with starting message"
					run:  (#gerrit.#setCodeReview & {
						#args: message: "Started the build... see progress at ${{ github.event.repository.html_url }}/actions/runs/${{ github.run_id }}"
					}).res
				},
			]
		}
		test: {
			needs:     "start"
			strategy:  #testStrategy
			"runs-on": "${{ matrix.os }}"
			steps: [
				#setCUEEnv,
				#writeCookiesFile,
				#installGo,
				#checkoutCode,
				#cacheGoModules,
				#goGenerate,
				#goTest,
				#goTestRace,
				#goReleaseCheck,
				#checkGitClean,
				#pullThroughProxy,
				#step & {
					if:   "${{ env.CUE_IS_BUILD_BRANCH == 'true' && failure() }}"
					name: "Post any failures for this matrix entry"
					run:  (#gerrit.#setCodeReview & {
						#args: {
							message: "Build failed for ${{ runner.os }}-${{ matrix.go-version }}; see ${{ github.event.repository.html_url }}/actions/runs/${{ github.run_id }} for more details"
							labels: {
								"Code-Review": -1
							}
						}
					}).res
				},
			]
		}
		end: {
			"runs-on": "ubuntu-latest"
			steps: [
				#setCUEEnv,
				#writeCookiesFile,
				#step & {
					if:   "${{ env.CUE_IS_BUILD_BRANCH == 'true' }}"
					name: "Update Gerrit CL message with success message"
					run:  (#gerrit.#setCodeReview & {
						#args: {
							message: "Build succeeded for ${{ github.event.repository.html_url }}/actions/runs/${{ github.run_id }}"
							labels: {
								"Code-Review": 1
							}
						}
					}).res
				},
			]
			needs: "test"
		}
		tidyup: {
			"runs-on": "ubuntu-latest"
			steps: [
				#setCUEEnv,
				#step & {
					if:   "${{ env.CUE_IS_BUILD_BRANCH == 'true' }}"
					name: "Delete build branch"
					run: #"""
					branch=$(echo $GITHUB_REF | sed -e 's/refs\/heads\///')
					git push https://github.com/$GITHUB_REPOSITORY :$branch
					"""#
				},
			]
			needs: "test"
			if:    "always()"
		}
	}
}

test_dispatch: json.#Workflow & {
	env: CUECKOO_PAT: "${{ secrets.cueckooPAT }}"
	name: "Dispatch build branch"
	on: ["repository_dispatch"]
	defaults: run: shell: "bash"
	jobs: {
		create_build_branch: {
			"runs-on": "ubuntu-latest"
			steps: [
				#checkoutCode,
				#step & {
					name: "Checkout ref"
					run: """
					  git fetch https://cue-review.googlesource.com/cue ${{ github.event.client_payload.ref }}
					  git checkout FETCH_HEAD
					  """
				},
				#step & {
					name: "Create build branch"
					run: #"""
					git config user.email cueckoo@cuelang.org
					git config user.name cueckoo
					git config http.https://github.com/.extraheader "AUTHORIZATION: basic $(echo -n cueckoo:$CUECKOO_PAT | base64)"
					ref=$(echo ${{ github.event.client_payload.ref }} | sed -e 's/\//_/g')
					branch="$(date -u +%Y%m%d%H%M%S)-${{ github.event.client_payload.changeID }}-${{ github.event.client_payload.commit }}-$ref"
					git checkout -b $branch
					git push origin $branch
					"""#
				},
			]
		}
	}
}

release: {
	name: "Release"
	on: push: tags: ["v*"]
	jobs: {
		goreleaser: {
			"runs-on": "ubuntu-latest"
			steps: [{
				name: "Checkout code"
				uses: "actions/checkout@v2"
			}, {
				name: "Unshallow" // required for the changelog to work correctly.
				run:  "git fetch --prune --unshallow"
			}, {
				name: "Run GoReleaser"
				env: GITHUB_TOKEN: "${{ secrets.ACTIONS_GITHUB_TOKEN }}"
				uses: "docker://goreleaser/goreleaser:latest"
				with: args: "release --rm-dist"
			}]
		}
		docker: {
			name:      "docker"
			"runs-on": "ubuntu-latest"
			steps: [{
				name: "Check out the repo"
				uses: "actions/checkout@v2"
			}, {
				name: "Set version environment"
				run: """
					CUE_VERSION=$(echo ${GITHUB_REF##refs/tags/v})
					echo \"CUE_VERSION=$CUE_VERSION\"
					echo \"::set-env name=CUE_VERSION::$(echo $CUE_VERSION)\"
					"""
			}, {
				name: "Push to Docker Hub"
				env: {
					DOCKER_BUILDKIT: 1
					GOLANG_VERSION:  1.14
					CUE_VERSION:     "${{ env.CUE_VERSION }}"
				}
				uses: "docker/build-push-action@v1"
				with: {
					tags:           "${{ env.CUE_VERSION }},latest"
					repository:     "cuelang/cue"
					username:       "${{ secrets.DOCKER_USERNAME }}"
					password:       "${{ secrets.DOCKER_PASSWORD }}"
					tag_with_ref:   false
					tag_with_sha:   false
					target:         "cue"
					always_pull:    true
					build_args:     "GOLANG_VERSION=${{ env.GOLANG_VERSION }},CUE_VERSION=v${{ env.CUE_VERSION }}"
					add_git_labels: true
				}
			}]
		}
	}
}

rebuild_tip_cuelang_org: json.#Workflow & {
	name: "Push to tip"
	on: push: branches: ["master"]
	jobs: push: {
		"runs-on": "ubuntu-latest"
		steps: [{
			name: "Rebuild tip.cuelang.org"
			run:  "curl -f -X POST -d {} https://api.netlify.com/build_hooks/${{ secrets.CuelangOrgTipRebuildHook }}"
		}]
	}
}
