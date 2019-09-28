package home

import "tool/exec"

command run: runBase & {
	task echo cmd: "echo \(message)"
}

command run_list: runBase & {
	task echo cmd: ["echo", message]
}

command errcode: {
	task bad: exec.Run & {
		kind:   "exec"
		cmd:    "ls --badflags"
		stderr: string // suppress error message
	}}

// TODO: capture stdout and stderr for tests.
command runRedirect: {
	task echo: exec.Run & {
		kind: "exec"
		cmd:  "echo \(message)"
	}
}

command baddisplay: {
	task display: {
		kind: "print"
		text: 42
	}
}

command http: {
	task testserver: {
		kind: "testserver"
		url:  string
	}
	task http: {
		kind:   "http"
		method: "POST"
		url:    task.testserver.url

		request body:  "I'll be back!"
		response body: string // TODO: allow this to be a struct, parsing the body.
	}
	task print: {
		kind: "print"
		text: task.http.response.body
	}
}

command print: {
	task: {
		t1: exec.Run & {
			cmd: ["sh", "-c", "sleep 1; echo t1"]
			stdout: string
		}
		t2: exec.Run & {
			cmd: ["sh", "-c", "sleep 1; echo t2"]
			stdout: string
		}
		t3: cli.Print & {
			text: (f & {arg: t1.stdout + t2.stdout}).result
		}
	}
}

f :: {
    arg: string
    result: strings.Join(strings.Split(arg, ""), ".")
}