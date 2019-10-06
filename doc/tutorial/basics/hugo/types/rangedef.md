+++
title = "Predefined Ranges"
description = ""
weight = 2075
layout = "tutorial"
+++
CUE numbers have arbitrary precision.
Also there is no unsigned integer type.

CUE defines the following predefined identifiers to restrict the ranges of
integers to common values.

```
uint      >=0
uint8     >=0 & <=255
int8      >=-128 & <=127
uint16    >=0 & <=65536
int16     >=-32_768 & <=32_767
rune      >=0 & <=0x10FFFF
uint32    >=0 & <=4_294_967_296
int32     >=-2_147_483_648 & <=2_147_483_647
uint64    >=0 & <=18_446_744_073_709_551_615
int64     >=-9_223_372_036_854_775_808 & <=9_223_372_036_854_775_807
int128    >=-170_141_183_460_469_231_731_687_303_715_884_105_728 &
              <=170_141_183_460_469_231_731_687_303_715_884_105_727
uint128   >=0 & <=340_282_366_920_938_463_463_374_607_431_768_211_455
```


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>range.cue</i>
<p>
{{< highlight go >}}
positive: uint
byte:     uint8
word:     int32

{
    a: positive & -1
    b: byte & 128
    c: word & 2_000_000_000
}

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval -i range.cue</i>
<p>
{{< highlight go >}}
a: _|_ // invalid value -1 (out of bound int & >=0)
b: 128
c: 2000000000
{{< /highlight >}}
</div>
</section>