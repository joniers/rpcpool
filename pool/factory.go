package pool

// PooledObjectFactory is factory interface for ObjectPool
type PooledObjectFactory interface {

	/**
	 * Create an instance that can be served by the pool and wrap it in a
	 * PooledObject to be managed by the pool.
	 *
	 * return error if there is a problem creating a new instance,
	 *    this will be propagated to the code requesting an object.
	 */
	MakeObject() (*PooledObject, error)

	/**
	 * Destroys an instance no longer needed by the pool.
	 */
	DestroyObject(object *PooledObject) error

	/**
	 * Ensures that the instance is safe to be returned by the pool.
	 *
	 * return false if object is not valid and should
	 *         be dropped from the pool, true otherwise.
	 */
	ValidateObject(object *PooledObject) bool

	/**
	 * Reinitialize an instance to be returned by the pool.
	 *
	 * return error if there is a problem activating object,
	 *    this error may be swallowed by the pool.
	 */
	ActivateObject(object *PooledObject) error

	/**
	 * Uninitialize an instance to be returned to the idle object pool.
	 *
	 * return error if there is a problem passivating obj,
	 *    this exception may be swallowed by the pool.
	 */
	PassivateObject(object *PooledObject) error
}
