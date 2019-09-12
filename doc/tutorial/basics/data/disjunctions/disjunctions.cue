conn: {
    address:  string
    port:     int
    protocol: "tcp" | "udp"
}

lossy: conn & {
    address:  "1.2.3.4"
    port:     8888
    protocol: "udp"
}