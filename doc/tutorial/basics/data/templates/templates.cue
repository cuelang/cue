// The following struct is unified with all elements in job.
// The name of each element is bound to Name and visible in the struct.
job <Name>: {
    name:     Name
    replicas: uint | *1
    command:  string
}

job list command: "ls"

job nginx: {
    command:  "nginx"
    replicas: 2
}