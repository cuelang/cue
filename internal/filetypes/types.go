// Code generated by gocode.Generate; DO NOT EDIT.

package filetypes

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/encoding/gocode/gocodec"
	_ "cuelang.org/go/pkg"
)

var cuegenCodec, cuegenInstance = func() (*gocodec.Codec, *cue.Instance) {
	var r *cue.Runtime
	r = &cue.Runtime{}
	instances, err := r.Unmarshal(cuegenInstanceData)
	if err != nil {
		panic(err)
	}
	if len(instances) != 1 {
		panic("expected encoding of exactly one instance")
	}
	return gocodec.New(r, nil), instances[0]
}()

// cuegenMake is called in the init phase to initialize CUE values for
// validation functions.
func cuegenMake(name string, x interface{}) cue.Value {
	f, err := cuegenInstance.Value().FieldByName(name, true)
	if err != nil {
		panic(fmt.Errorf("could not find type %q in instance", name))
	}
	v := f.Value
	if x != nil {
		w, err := cuegenCodec.ExtractType(x)
		if err != nil {
			panic(err)
		}
		v = v.Unify(w)
	}
	return v
}

// Data size: 1623 bytes.
var cuegenInstanceData = []byte("\x01\x1f\x8b\b\x00\x00\x00\x00\x00\x00\xff\xacW\xddo\xd4H\x12\xb7CN:\xb7\xb8\x93N\xe2\xf5\xa4\xc2H\x88\x8b8G< \xd0H\x11\x02\x02\xa7\xbc\x1c\xa7\x13\xfb\x84P\xd4c\x97gz\xb1\xbb\xbd\xee6$\"\xf3\xb0\xbb,\xbb\u007f5\xb3\xaa\xee\xf6\u760f\xec&/\x19\xd7WWU\xf7\xaf>\xfe\xb6\xfdu/\xdc\xdb\xfe\x16\x84\u06df\x82\xe0\xc1\xf6\xc7kax]Hm\xb8L\xf1\x98\x1bN\xf4\xf0Z\xb8\xff\u007f\xa5L\xb8\x17\x84\xfb\xff\xe3f\x1d^\x0f\u00bf<\x17\x05\xeap\xfb1\b\x82\u007fn\u007f\xd9\v\u00ff\xbfz\x9d6\x98\xe4\xa2\xf0\x9a\x1f\x83p\xfb!\b\xeel\u007f\xbe\x16\x86\u007f\xed\xe9\x1f\x82p/\xdc\xff//\x91\f\xed[\"\v\x82\xe0\u04cd\x03\xf2$\f\xf7\xc202\xe7\x15\xea$m0\xfct\xe3\x1f\x15O\xdf\xf0\x15\u00b2\x11E\xc6\xd8\xe1!<\x06:\x1fRU\u05e8+%3\rF\x01\x87\xff('\x94\x10;a\xb7\xe8\xdf\x02\u07b3\x88\x8e\x97\xbc\xc4\x05\xf8?mj!W,B\x99\xaaL\xc8U\u01f8\xf5\xccSX$\xa4\xc1\xba\xaa\xd1p#\x94|\xb4\x80['#\n\x8brU\x97\x8f:U\xd2~\xae\xea\x92E\x86\xaf\xf4#{p\xf4\u029d\xf4z\xd1\x1d\xb9a\x1b\x1b\xc41\xe6\xbc)\f\b\rf\x8d@.B\xa31\x83\\\u0560M&$p\x99\xd1/\u0558\x04^\xae\x114\x1a#\xe4JC\x86\x15\u028c\xac(\xd9k\x97*\xa3\xa8\xbd\xe1\x05\xd8\xf8\xe1\xf68\x01\a\xf1\xbfc\xb8h\xbd\xd9\f\xf2y\"s\x05\x19\xe6B\xa2\x86\xb5z\a\u0719\x15\x1al\x9a0\xb3\x0eui\xc1\u0327\x98\x14m\xb4\xf6\x8bE\x197\xbc\xcf\u0281\xa9\x1b\x84\v\xc8y\xa1\x91E5\xe6X\xa3LQ/v\x99\xe9yZ8\u018c\xa6uMP\xe6Ib\xa9T\xc1\"U\xd17/\x9c\x8a\xa3\xa5JjSs!M/\xf7\x06\xb1\xf2y\xd1\vO\x132UeU\xa0\xb1\xcf\xc2\xd3\xcaJ\u0566\xf5\xc0\u0474\xa9\x91\x97\xadS\x8e\x96\xa9T\xf7!:\x1a7\xa6\x16\xcb\u01b8\x00,\u0365\x97\xeeE\xd3\xe5\xd1\xc59\x1f\xec%g\"\xb7\xb90\xa0*\xac\xb9\x8b\xc4I'\xec\xf0\x90T_\xaeQ#\x18,\xab\x82\x1b\xd4\xc0k\xb4\x17 \xe96\x8c\x82%B#E.\x90\xee\x05\xb8\xb1\x8f\xa1V\u0280\xca\xc1\xac\x85&#\xa9\x92\xb9X5\ue104\xd9\x03\xec}\tY5\u01bd\xd3\xfe\xd5\xd0\xd7\x00\x17\aq\xda \xbd\x98S\xa2'I\u00a2h\u00e2\xa8@\x03gp\xe4\u0107\xe9\x98\xdcZ4\xca\u02d4I\x96\x06o\xe8\x8c\xf5Gk\xefJ\xda\u042b%\xa8\xe9D\xa7k,\xb9w\x86t\xf1\u0320\xd4\xeeIX\xe98\xf9^+\x19\xfb\xaf\t\x86)\x1a\xde\x18\u0545\xb3q*\xe7\xbc,.\xabr9\x8d\r\xe1>\xc23z]_M\xb8\x8d\xe03\x19?\xbd7\x97s\x9f\u0543\u065cO\x99\u04dc\x9f\xde\xfbJ\xd6\t\xcf}\xce7,RMe\u0687\xe3\xbczx\xff\xea\xddzx\xff\xb2~\xe1[\xaa\x04\u007f\xfc9\x9f>y|\xf5a<y\xfc\x950rA\xb0\x1f\u0191a\xfe\xa7\u00b8\xff\xe0\xe9\xc3+\x87\xa6\xb5zI|\xb6\xbd\xeeY\vS(y\xa5][\xe9\xa1K\x85\xcc\x17F\u01eaj*\x88FP\x1d\x9c <\x8e\xbb\xb2{\u02a2\x98f\x04G\xa1\x9eK_\xac/\x01\x9eH_-\x95@\xdbS\v\"\x17\x99\x17\x1f\x93\xe5<\u0657\no\x84\xbeXW\r\xa6Tsf\x06T\x83g\x86\xa8+\xe5C\xb0\u0515\"ZU+\xa3:\xd7\xec\x17\x8b\xa8\xfc\xbf8~\xb1\x00:\\\xe3\x0fw-)\xb6\x86Z\x85\xcer\xb5\xf4\xdcj\xd9g\xc8r\x97BV\u02ee\u0477\xe3\r\b\x99\x89\xd4\xf5\x14\x97t\xbaAnlc\xaa\xb1\xaaQ\xa3\xa4a\x038]\u01ea\xe6e\u00ba\xe1h\x017\x8f\xe2\u0619\x940\x1e\x8b C\x83u9\x98\"R\xac\r\x17\xb2\xb5\x03z\xad\x9a\"\xa3\xde5\x9a%\x0e\x0f\u1e6a\xa1\x1d@\xef\x82\xc5w\xc9\xcf'\x92\xc0\xa9\x8f\xea\xb4\x16K\xe7\x9f{uw\xe1\xddZ\xa4k\x10Fc\x91\u06fe\xc7%\xa9\xa6J\xbe\xc5\u06b8\x86\xc9\xe1\xe9w\u03fcF\xc2&\x13]7\xa4\xd99n8\xd8yzn\a\xca\xd1\xc0\xd7\x0eN\x931+\u0395r\xef\u040d\x89N+v\a\xc7\xfe:\xe8~\x1c\"RU\x964\\\x15B\xa2#\x1b\xb5\x8b\x05bX\x1483\x0e\x80\xcezg\x99`\xb7\xaay\xb5\x1eq-\xc513\xbe\x1a\xb12\xbej\x19\x86O8\xc6\x1b\xb4\x18\u007f\u03c6\x15\xc8\x16 \u02e4(w\xb8>t\xcf.f\xf9\x85\x13 \xb8\xec\xf0-\xce,\xdb>\xf5\x1d\xbe\x03\x80\x15\xa0g\xef \x10/\xa0\xabN=L\x9c\x84\x85\x01A\xa3\x97 \x92\x13 \u065d#\x88\x18w\u0670\xd7\xd7gd\xb5\xe3\x92\xfb\x8biP&\xad\xe9P\x10\x13\xb1\xbb\xc0(*\xb8=dEA\xf8\xb2O\xaaWb\xb5]5\xbc]\x9aF\x1c\u007fG\xdd\x0e*3\a\x8e\x86\x10\u007f\x89\xc3G\xb7c\xa8\x17\xf8\x16s\xaaB\xc9+\xf1\x19[\x9e\xfb\r\x86\x1c\x8cl\xef\xe96\x17\u07c3\xa8\x8e\xf1\xa2p\xcc\x04N\fd\n5He@\u0234h2t\x8b\x93\xaaK89N\x98\x95\xb3\x0e\u0675\x8d\x16\u0523nw\xeb`n\xbd\xa7\x1et:\a\xc2n\xe5i\xd1\b\x17\x10\xdb\xf6n\u007f\xb5 \x9cl\x14\xd3\tb\xbc\x97L\xdb\xf2x\v\x9ar\xc7\xfb\u041d\x11\xfb_p{Ja\xd1d[\x9a\xda\x1b\xefMS\xeex[\x9ap7T\x0ee;\x90\r\u70dd|\xf9\x1c\xed\x9c7\x1fUo\u007f\xa7\xce\xf5\x17\xe0rMY\xa7\xfa\xe6\xfe[\xecN\xb6S\xf2y'\xe7\xf3\xb9\xfe\xa27\x93<\xce\xe7o>o}<\xa3\u04ac\x13\x1b\xc3 \xb6\x9bG\xfd\x13j7\xe5\xa1\xf2\xb0|\xd3t\xbc\x9a\xe6\xe5\u646f\xf6co[\xb7F\xaby\x17\xd7p%\x9f\r`6/\x9d_\x1b6\x1e\x18\xbbV\u0482\xa0\x8f\xa0o$\xfd|?A\x8b\x03\t\\\xb4\xf76\x9cn[?\x86Cmo\xbc\xef2\xe3\xe4\x8e\xdc \x18:\xcb\xe3\xc65\xebO'\xd8w\x8fY\xb9\xde\a\xa3\xca/\x19\xec\x05\a=o\x02\x9c\xf9\x16\xd8w\x8e\x89\xf8\x8e\xe9\r\x1b\x97\xdbK\x94<;q\xbb^2>e\xda\x1c>\xeb\xf2\x17\xdb\xc0\xb7jmX\x10\xfc\x1e\x00\x00\xff\xffB\xee2\xf0\xba\x14\x00\x00")