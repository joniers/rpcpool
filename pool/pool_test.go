package pool

import (
	"fmt"
	"os"
	"push_manager"
	"testing"
	"time"

	"git.apache.org/thrift.git/lib/go/thrift"
)

type MyObjectFactory struct {
	host           string
	port           int32
	mode           int32
	socket_timeout time.Duration
}

func (f *MyObjectFactory) MakeObject() (*PooledObject, error) {
	var trans thrift.TTransport
	var err error
	trans, err = thrift.NewTSocketTimeout("11.12.112.250:5818", 2000*time.Millisecond)
	if err != nil {
		return nil, err
	}
	trans = thrift.NewTFramedTransport(trans)
	var protocolFactory thrift.TProtocolFactory
	protocolFactory = thrift.NewTBinaryProtocolFactoryDefault()
	client := push_manager.NewPushManagerStubClientFactory(trans, protocolFactory)
	if err = client.Transport.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "Error opening socket to ", "11.12.112.250", ":", 5818, " ", err)
		return nil, err
	}
	return NewPooledObject(client), nil
}

func (f *MyObjectFactory) DestroyObject(object *PooledObject) error {
	//	fmt.Printf("****** into DestroyObject")
	var cli *push_manager.PushManagerStubClient = object.Object.(*push_manager.PushManagerStubClient)
	var err error
	err = cli.Transport.Close()
	if err != nil {
		return err
	}
	return err
}

func (f *MyObjectFactory) ValidateObject(object *PooledObject) bool {
	cli := object.Object.(*push_manager.PushManagerStubClient)
	res := cli.Transport.IsOpen()
	return res
}

func (f *MyObjectFactory) ActivateObject(object *PooledObject) error {
	return nil
}
func (f *MyObjectFactory) PassivateObject(object *PooledObject) error {
	return nil
}

var (
	debugTest = false
)
var perf bool

/*
获取对象池对象，看超过数量后还能否获取到对象
*/
//func TestGet(t *testing.T) {
//	var poolsize int = 100
//	factory := MyObjectFactory{"11.12.112.250", 5818, 3, 2000}
//	pool := NewConnPool(poolsize, 3*time.Second, 3*time.Second, 3600*time.Second, &factory)
//	cn, err := pool.Get()
//	if err != nil {
//		t.Log("get resource failed!")
//		t.Fail()
//	}
//	var cli *push_manager.PushManagerStubClient = cn.Object.(*push_manager.PushManagerStubClient)
//	res, er := cli.Echo(-1, "tester", "hello", "")
//	if er != nil {
//		fmt.Fprintln(os.Stderr, "get Echo err:", er)
//		t.Log("get Echo failed!")
//		t.Fail()
//	}
//	fmt.Printf("*** result:%s,pool size:%d\n", res.Value, pool.Len())
//	pool.Return(cn)
//	for i := 0; i < 1000; i++ {
//		//		fmt.Printf("***index:%d\n", i)
//		cn, er = pool.Get()
//		if cn == nil || er != nil {
//			fmt.Println("can't get resource from pool!")
//			fmt.Println(er)
//			break
//		}
//		//		pool.Return(cn)
//		//		fmt.Printf("***pool size:%d\n", pool.Len())
//	}
//	fmt.Printf("***pool size:%d\n", pool.Len())
//	//	pool.Return(cn)
//	pool.Close()
//}

///*
//归还对象池对象，看空闲对象数量跟归还数量是否一致
//*/
//func TestPut(t *testing.T) {
//	var poolsize int = 100
//	factory := MyObjectFactory{"11.12.112.250", 5818, 3, 2000}
//	pool := NewConnPool(poolsize, 3*time.Second, 3*time.Second, 3600*time.Second, &factory)
//	var conns []*PooledObject
//	for i := 0; i < 50; i++ {
//		//		fmt.Printf("***index:%d\n", i)
//		cn, er := pool.Get()
//		if cn == nil || er != nil {
//			fmt.Println("can't get resource from pool!")
//			fmt.Println(er)
//			break
//		}
//		conns = append(conns, cn)
//	}
//	for i := 0; i < len(conns); i++ {
//		pool.Return(conns[i])
//	}
//	fmt.Printf("***pool free size:%d\n", pool.FreeLen())
//	if pool.FreeLen() != 50 {
//		t.Fail()
//	}
//	pool.Close()
//}

///*
//	单元测试，用来验证对象超期逐出后，对象数量是否为0
//*/
//func TestCheckIdleTimeout(t *testing.T) {
//	var poolsize int = 10
//	factory := MyObjectFactory{"11.12.112.250", 5818, 3, 2000}
//	var conns []*PooledObject
//	pool := NewConnPool(poolsize, 2*time.Second, 2*time.Second, 3*time.Second, &factory)
//	for i := 0; i < 10; i++ {
//		cn, er := pool.Get()
//		if cn == nil || er != nil {
//			fmt.Println("can't get resource from pool!")
//			fmt.Println(er)
//			break
//		}
//		conns = append(conns, cn)
//	}
//	for i := 0; i < len(conns); i++ {
//		pool.Return(conns[i])
//	}
//	fmt.Printf("*** pool size:%d free size:%d\n", pool.Len(), pool.FreeLen())
//	time.Sleep(10 * time.Second)
//	fmt.Printf("*** pool size:%d free size:%d\n", pool.Len(), pool.FreeLen())
//	if pool.FreeLen() != 0 {
//		t.Fail()
//	}
//	pool.Close()
//}

///*
//	单元测试，用来验证对象超期逐出后，重新申请对象，看是否可以申请到制定数量的对象
//*/
//func TestCheckIdleTimeoutandGet(t *testing.T) {
//	var poolsize int = 10
//	factory := MyObjectFactory{"11.12.112.250", 5818, 3, 2000}
//	var conns []*PooledObject
//	pool := NewConnPool(poolsize, 2*time.Second, 2*time.Second, 3*time.Second, &factory)
//	for i := 0; i < 10; i++ {
//		//		fmt.Printf("***index:%d\n", i)
//		cn, er := pool.Get()
//		if cn == nil || er != nil {
//			fmt.Println("can't get resource from pool!")
//			fmt.Println(er)
//			break
//		}
//		conns = append(conns, cn)
//	}
//	for i := 0; i < len(conns); i++ {
//		pool.Return(conns[i])
//	}
//	time.Sleep(10 * time.Second)
//	fmt.Printf("*** pool size:%d free size:%d\n", pool.Len(), pool.FreeLen())
//	for i := 0; i < 5; i++ {
//		cn, er := pool.Get()
//		if cn == nil || er != nil {
//			fmt.Println("can't get resource from pool!")
//			fmt.Println(er)
//			break
//		}
//	}
//	fmt.Printf("*** pool size:%d\n", pool.Len())
//	if pool.Len() != 5 {
//		t.Fail()
//	}
//	pool.Close()
//}

///*
//	单元测试，正常获取对象后，close对象池，Get失败
//*/
//func TestCheckGetandCloseGet(t *testing.T) {
//	var poolsize int = 10
//	factory := MyObjectFactory{"11.12.112.250", 5818, 3, 2000}
//	pool := NewConnPool(poolsize, 2*time.Second, 2*time.Second, 3*time.Second, &factory)
//	cn, er := pool.Get()
//	if cn == nil || er != nil {
//		fmt.Println("can't get resource from pool!")
//		fmt.Println(er)
//		t.Error("failed!")
//	}
//	pool.Close()
//	fmt.Printf("*** close pool\n")
//	cn, er = pool.Get()
//	if cn == nil || er != nil {
//		fmt.Println("can't get resource from pool!")
//		fmt.Println(er)
//	}
//}
func BenchmarkGet(b *testing.B) {
	var poolsize int = 10
	factory := MyObjectFactory{"11.12.112.250", 5818, 3, 2000}
	pool := NewConnPool(poolsize, 2*time.Second, 2*time.Second, 3*time.Second, &factory)
	for i := 0; i < b.N; i++ {
		cn, er := pool.Get()
		if cn == nil || er != nil {
			fmt.Println("can't get resource from pool!")
			fmt.Println(er)
			break
		}
		var cli *push_manager.PushManagerStubClient = cn.Object.(*push_manager.PushManagerStubClient)
		_, er = cli.Echo(-1, "tester", "hello", "")
		if er != nil {
			fmt.Fprintln(os.Stderr, "get Echo err:", er)
			break
		}
		pool.Return(cn)
	}
	//	fmt.Println("BenchmarkGet finish!")
}
