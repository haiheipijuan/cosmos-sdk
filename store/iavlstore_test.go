package store

import (
	"testing"

	"github.com/stretchr/testify/assert"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/iavl"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	cacheSize        = 100
	numHistory int64 = 5
)

var (
	treeData = map[string]string{
		"hello": "goodbye",
		"aloha": "shalom",
	}
	nMoreData = 0
)

// make a tree and save it
func newTree(t *testing.T, db dbm.DB) (*iavl.VersionedTree, CommitID) {
	tree := iavl.NewVersionedTree(db, cacheSize)
	for k, v := range treeData {
		tree.Set([]byte(k), []byte(v))
	}
	for i := 0; i < nMoreData; i++ {
		key := cmn.RandBytes(12)
		value := cmn.RandBytes(50)
		tree.Set(key, value)
	}
	hash, ver, err := tree.SaveVersion()
	assert.Nil(t, err)
	return tree, CommitID{ver, hash}
}

func TestIAVLStoreGetSetHasDelete(t *testing.T) {
	db := dbm.NewMemDB()
	tree, _ := newTree(t, db)
	iavlStore := newIAVLStore(tree, numHistory)

	key := "hello"

	exists := iavlStore.Has([]byte(key))
	assert.True(t, exists)

	value := iavlStore.Get([]byte(key))
	assert.EqualValues(t, value, treeData[key])

	value2 := "notgoodbye"
	iavlStore.Set([]byte(key), []byte(value2))

	value = iavlStore.Get([]byte(key))
	assert.EqualValues(t, value, value2)

	iavlStore.Delete([]byte(key))

	exists = iavlStore.Has([]byte(key))
	assert.False(t, exists)
}

func TestIAVLIterator(t *testing.T) {
	db := dbm.NewMemDB()
	tree, _ := newTree(t, db)
	iavlStore := newIAVLStore(tree, numHistory)
	iter := iavlStore.Iterator([]byte("aloha"), []byte("hellz"))
	expected := []string{"aloha", "hello"}
	for i := 0; iter.Valid(); iter.Next() {
		expectedKey := expected[i]
		key, value := iter.Key(), iter.Value()
		assert.EqualValues(t, key, expectedKey)
		assert.EqualValues(t, value, treeData[expectedKey])
		i += 1
	}
}

func TestIAVLStoreQuery(t *testing.T) {
	db := dbm.NewMemDB()
	tree := iavl.NewVersionedTree(db, cacheSize)
	iavlStore := newIAVLStore(tree, numHistory)

	k, v := []byte("wind"), []byte("blows")
	k2, v2 := []byte("water"), []byte("flows")
	v3 := []byte("is cold")
	// k3, v3 := []byte("earth"), []byte("soes")
	// k4, v4 := []byte("fire"), []byte("woes")

	cid := iavlStore.Commit()
	ver := cid.Version
	query := abci.RequestQuery{Path: "/key", Data: k, Height: ver}

	// set data without commit, doesn't show up
	iavlStore.Set(k, v)
	qres := iavlStore.Query(query)
	assert.Equal(t, uint32(sdk.CodeOK), qres.Code)
	assert.Nil(t, qres.Value)

	// commit it, but still don't see on old version
	cid = iavlStore.Commit()
	qres = iavlStore.Query(query)
	assert.Equal(t, uint32(sdk.CodeOK), qres.Code)
	assert.Nil(t, qres.Value)

	// but yes on the new version
	query.Height = cid.Version
	qres = iavlStore.Query(query)
	assert.Equal(t, uint32(sdk.CodeOK), qres.Code)
	assert.Equal(t, v, qres.Value)

	// modify
	iavlStore.Set(k2, v2)
	iavlStore.Set(k, v3)
	cid = iavlStore.Commit()

	// query will return old values, as height is fixed
	qres = iavlStore.Query(query)
	assert.Equal(t, uint32(sdk.CodeOK), qres.Code)
	assert.Equal(t, v, qres.Value)

	// update to latest in the query and we are happy
	query.Height = cid.Version
	qres = iavlStore.Query(query)
	assert.Equal(t, uint32(sdk.CodeOK), qres.Code)
	assert.Equal(t, v3, qres.Value)
	query2 := abci.RequestQuery{Path: "/key", Data: k2, Height: cid.Version}
	qres = iavlStore.Query(query2)
	assert.Equal(t, uint32(sdk.CodeOK), qres.Code)
	assert.Equal(t, v2, qres.Value)

	// default (height 0) will show latest -1
	query0 := abci.RequestQuery{Path: "/store", Data: k}
	qres = iavlStore.Query(query0)
	assert.Equal(t, uint32(sdk.CodeOK), qres.Code)
	assert.Equal(t, v, qres.Value)
}
