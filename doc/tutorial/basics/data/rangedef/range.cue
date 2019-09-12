positive: uint
byte:     uint8
word:     int32

{
    a: positive & -1
    b: byte & 128
    c: word & 2_000_000_000
}