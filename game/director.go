package game

type Director interface {
	/**
	 * Initialize the director
	 */
	Init(*Board)

	/**
	 * Perform a single step of actions
	 */
	Act()

	/**
	 * Continue acting periodically, until End() is called
	 */
	ActContinuously()

	/**
	 * Stop acting
	 */
	End()
}
