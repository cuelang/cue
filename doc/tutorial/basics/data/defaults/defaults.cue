// any positive number, 1 is the default
replicas: uint | *1

// the default value is ambiguous
protocol: *"tcp" | "udp"
protocol: *"udp" | "tcp"