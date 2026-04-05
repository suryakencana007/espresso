package espresso

// LayerStack holds a collection of LayerConfigs for reuse.
// LayerConfigs are type-erased and can be shared across handlers
// with different Req/Res types.
//
// Example:
//
//	// Define common layers once
//	commonLayers := espresso.Layers(
//	    espresso.Timeout(5*time.Second),
//	    espresso.Logging(logger, "api"),
//	)
//
//	// Reuse across routes
//	app.Post("/users", espresso.WithLayers(createUser, commonLayers...))
//	app.Post("/posts", espresso.WithLayers(createPost, commonLayers...))
type LayerStack []LayerConfig

// Layers creates a reusable stack of layer configurations.
// Layers are applied in order (first added = outermost).
// Similar to Tower's ServiceBuilder pattern.
//
// Example:
//
//	common := espresso.Layers(
//	    espresso.Timeout(5*time.Second),
//	    espresso.Logging(logger, "api"),
//	    espresso.Metrics(collector, "api"),
//	)
//
//	app.Post("/users", espresso.WithLayers(createUser, common...))
//	app.Post("/posts", espresso.WithLayers(createPost, common...))
func Layers(configs ...LayerConfig) LayerStack {
	return LayerStack(configs)
}

// Combine merges two LayerStacks into one.
// Layers from both stacks are concatenated (first all from `s`, then all from `other`).
//
// Example:
//
//	common := espresso.Layers(
//	    espresso.Timeout(5*time.Second),
//	    espresso.Logging(logger, "api"),
//	)
//
//	userLayers := espresso.Layers(
//	    espresso.Validation(userValidator),
//	)
//
//	allLayers := common.Combine(userLayers)
//	app.Post("/users", espresso.WithLayers(createUser, allLayers...))
func (s LayerStack) Combine(other LayerStack) LayerStack {
	return append(s, other...)
}

// Append adds more LayerConfigs to the stack.
//
// Example:
//
//	common := espresso.Layers(
//	    espresso.Timeout(5*time.Second),
//	)
//
//	// Add more layers
//	common = common.Append(
//	    espresso.Logging(logger, "api"),
//	    espresso.Metrics(collector, "api"),
//	)
func (s LayerStack) Append(configs ...LayerConfig) LayerStack {
	return append(s, configs...)
}

// Prepend adds LayerConfigs to the beginning of the stack.
// Prepended layers become outermost (executed first).
//
// Example:
//
//	common := espresso.Layers(
//	    espresso.Timeout(5*time.Second),
//	)
//
//	// Add recovery as outermost layer
//	common = common.Prepend(
//	    espresso.Timeout(10*time.Second),  // Will wrap Timeout layer
//	)
func (s LayerStack) Prepend(configs ...LayerConfig) LayerStack {
	return append(configs, s...)
}
