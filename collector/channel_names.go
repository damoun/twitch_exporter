package collector

import "fmt"

// ChannelNames represents a list of twitch channels.
type ChannelNames []string

// IsCumulative is required for kingpin interfaces to allow multiple values
func (c ChannelNames) IsCumulative() bool {
	return true
}

// Set sets the value of a ChannelNames
func (c *ChannelNames) Set(v string) error {
	*c = append(*c, v)
	return nil
}

// String returns a string representation of the Channels type.
func (c ChannelNames) String() string {
	return fmt.Sprintf("%v", []string(c))
}
