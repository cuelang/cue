rn: >=3 & <8        // type int | float
ri: >=3 & <8 & int  // type int
rf: >=3 & <=8.0     // type float
rs: >="a" & <"mo"

{
    a: rn & 3.5
    b: ri & 3.5
    c: rf & 3
    d: rs & "ma"
    e: rs & "mu"

    r1: rn & >=5 & <10
}