package sdk

type ActivityContext interface {
}

type ActivityFunc func(ctx ActivityContext, input any) (any, error)
