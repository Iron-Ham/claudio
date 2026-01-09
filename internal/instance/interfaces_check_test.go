package instance

// Compile-time interface satisfaction checks.
// These declarations verify that concrete types implement their intended interfaces.
// If a type doesn't implement an interface, the compiler will report an error here.
var (
	// Verify RingBuffer implements OutputBuffer
	_ OutputBuffer = (*RingBuffer)(nil)

	// Verify Detector implements StateDetector
	_ StateDetector = (*Detector)(nil)

	// Verify MetricsParser implements MetricsParsing
	_ MetricsParsing = (*MetricsParser)(nil)
)
