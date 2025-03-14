package main

type substitution struct {
	key   variable
	value expression

	left   *substitution
	right  *substitution
	height int
}

func NewAVL() *substitution {
	return nil
}

func (n *substitution) copyNode() *substitution {
	return &substitution{
		key:    n.key,
		value:  n.value,
		left:   n.left,
		right:  n.right,
		height: n.height,
	}
}

func (n *substitution) Insert(k variable, v expression) *substitution {
	tree, _ := n.insert(k, v)
	return tree
}

// immutable update. boolean indicate actual new insertion happened
func (n *substitution) insert(k variable, v expression) (*substitution, bool) {
	if n == nil {
		return &substitution{key: k, value: v, height: 1}, true
	}
	if n.key == k {
		return n, false
	}
	if n.key < k {
		left, inserted := n.left.insert(k, v)
		if !inserted {
			return n, false
		}
		newn := n.copyNode()
		newn.left = left
		return newn.rebalance(), true
	}
	// n.key > k
	right, inserted := n.right.insert(k, v)
	if !inserted {
		return n, false
	}
	newn := n.copyNode()
	newn.right = right
	return newn.rebalance(), true
}

func (n *substitution) getHeight() int {
	if n == nil {
		return 0
	}
	return n.height
}

// we just copied n and its larger children, potentially causing inbalance
// these are therefore all safe to modify in this function
func (n *substitution) rebalance() *substitution {
	inbalance := n.right.getHeight() - n.left.getHeight()
	if inbalance < 2 && inbalance > -2 {
		n.resetHeight()
		return n
	}
	if inbalance == -2 { // left is higher
		child := n.left
		if child.left.getHeight() > child.right.getHeight() {
			n.left = child.right
			child.right = n
			n.resetHeight()
			child.resetHeight()
			return child
		}
		grandchild := child.right
		child.right = grandchild.left
		grandchild.left = child
		n.left = grandchild.right
		grandchild.right = n
		n.resetHeight()
		child.resetHeight()
		grandchild.resetHeight()
		return grandchild
	}
	// inbalance == 2, right is higher
	child := n.right
	if child.right.getHeight() > child.left.getHeight() {
		n.right = child.left
		child.left = n
		n.resetHeight()
		child.resetHeight()
		return child
	}
	grandchild := child.left
	child.left = grandchild.right
	grandchild.right = child
	n.right = grandchild.left
	grandchild.left = n
	n.resetHeight()
	child.resetHeight()
	grandchild.resetHeight()
	return grandchild
}

func (n *substitution) resetHeight() {
	n.height = max(n.left.getHeight(), n.right.getHeight()) + 1
}

func (n *substitution) Lookup(k variable) (expression, bool) {
	if n == nil {
		return nil, false
	}
	switch {
	case n.key < k:
		return n.left.Lookup(k)
	case n.key > k:
		return n.right.Lookup(k)
	}
	// n.value == k
	return n.value, true
}
