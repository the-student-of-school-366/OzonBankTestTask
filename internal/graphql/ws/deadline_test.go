package ws

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStaleReadTimerDoesNotCloseRenewedDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	old := time.NewTimer(time.Hour)
	defer old.Stop()
	conn := &connection{ctx: ctx, cancel: cancel, readTimer: old}
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Hour)))
	defer conn.readTimer.Stop()
	// Имитируем callback старого таймера, уже запущенный до его остановки,
	// но получивший mutex после установки нового срока.
	conn.mu.Lock()
	expired := conn.takeReadDeadlineLocked(old)
	conn.mu.Unlock()
	require.False(t, expired)
	require.NoError(t, ctx.Err())
}
