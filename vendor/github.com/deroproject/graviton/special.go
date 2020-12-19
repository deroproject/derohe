package graviton

//import "io"
//import "math"

import "golang.org/x/xerrors"


// we only have a keyhash and need to get both the key,value
func (t *Tree) GetKeyValue(keyhash [HASHSIZE]byte) (int,[]byte,[]byte, error) {
	return t.root.GetKeyValue(t.store, keyhash,256,0 )
}

func (in *inner) GetKeyValue(store *Store, keyhash [HASHSIZE]byte, valid_bit_count,used_bit_count int) (int,[]byte,[]byte, error) {
	if err := in.load_partial(store); err != nil { // if inner node is loaded partially, load it fully now
		return used_bit_count,nil,nil, err
	}

    if used_bit_count > valid_bit_count || valid_bit_count <= 0 {
        return used_bit_count,nil,nil, xerrors.Errorf("%w: right dead end at %d. keyhash %x", ErrNotFound, in.bit, keyhash)
    } 

	if isBitSet(keyhash[:], uint(in.bit)) {
		if in.right == nil {
			return used_bit_count,nil,nil, xerrors.Errorf("%w: right dead end at %d. keyhash %x", ErrNotFound, in.bit, keyhash)
		}
        switch  in.right.(type) { // draw left  branch
		case *inner:  return in.right.(*inner).GetKeyValue(store, keyhash,valid_bit_count,used_bit_count+1)
	    case *leaf: return in.right.(*leaf).GetKeyValue(store, keyhash,valid_bit_count,used_bit_count+1)
	     default:		panic("unknown node type")
	}
		
	}
	if in.left == nil {
		return used_bit_count,nil,nil, xerrors.Errorf("%w: left dead end at %d. keyhash %x", ErrNotFound, in.bit, keyhash)
	}
	switch in.left.(type) { // draw left  branch
		case *inner:  return in.left.(*inner).GetKeyValue(store, keyhash,valid_bit_count,used_bit_count+1)
	    case *leaf: return in.left.(*leaf).GetKeyValue(store, keyhash,valid_bit_count,used_bit_count+1)
	     default:		panic("unknown node type")
	}
}

// should we return a copy
func (l *leaf) GetKeyValue(store *Store, keyhash [HASHSIZE]byte,valid_bit_count,used_bit_count int) (int,[]byte, []byte, error) {
	if l.loaded_partial { // if leaf is loaded partially, load it fully now
		if err := l.loadfullleaffromstore(store); err != nil {
			return used_bit_count,nil,nil, err
		}
	}

	if l.keyhash == keyhash {
		return used_bit_count,l.key,l.value, nil
	}

	return used_bit_count,nil,nil, xerrors.Errorf("%w: collision, keyhash %x not found", ErrNotFound, keyhash)
}

