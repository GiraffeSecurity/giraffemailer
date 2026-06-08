// Package mailconn wraps go-imap/v2 with a simpler interface for archiving and
// cleanup operations.
package mailconn

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// Client is a connected, authenticated IMAP session.
type Client struct {
	c *imapclient.Client
}

// Connect dials the server, authenticates, and returns a ready Client.
func Connect(host string, port int, useTLS bool, username, password string) (*Client, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	var c *imapclient.Client
	var err error
	if useTLS {
		c, err = imapclient.DialTLS(addr, nil)
	} else {
		c, err = imapclient.DialInsecure(addr, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", addr, err)
	}
	if err := c.Login(username, password).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("login %s: %w", username, err)
	}
	return &Client{c: c}, nil
}

// Close gracefully logs out and closes the connection.
func (cl *Client) Close() error {
	if err := cl.c.Logout().Wait(); err != nil {
		cl.c.Close()
		return err
	}
	return nil
}

// MailboxInfo is a simplified summary of a LIST result.
type MailboxInfo struct {
	Name   string
	Delim  rune
	Flags  []imap.MailboxAttr
}

// ListMailboxes returns all mailboxes visible to the authenticated user.
func (cl *Client) ListMailboxes(ctx context.Context) ([]*MailboxInfo, error) {
	items, err := cl.c.List("", "*", nil).Collect()
	if err != nil {
		return nil, fmt.Errorf("list mailboxes: %w", err)
	}
	out := make([]*MailboxInfo, 0, len(items))
	for _, d := range items {
		if d == nil {
			continue
		}
		out = append(out, &MailboxInfo{
			Name:  d.Mailbox,
			Delim: d.Delim,
			Flags: d.Attrs,
		})
	}
	return out, nil
}

// SelectResult is the result of a SELECT command.
type SelectResult struct {
	NumMessages uint32
	UIDValidity uint32
	UIDNext     imap.UID
}

// Select opens a mailbox (shared read, not read-only so we can set \Deleted).
func (cl *Client) Select(ctx context.Context, name string) (*SelectResult, error) {
	d, err := cl.c.Select(name, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("select %q: %w", name, err)
	}
	return &SelectResult{
		NumMessages: d.NumMessages,
		UIDValidity: d.UIDValidity,
		UIDNext:     d.UIDNext,
	}, nil
}

// MessageMeta holds the ENVELOPE + size + flags fetched in Phase 1.
type MessageMeta struct {
	UID           imap.UID
	MessageID     string
	Subject       string
	SenderName    string
	SenderEmail   string
	Recipients    []Address
	Date          time.Time
	SizeBytes     int64
	Flags         []imap.Flag
	BodyStructure imap.BodyStructure
}

// Address is a simple name + email pair.
type Address struct {
	Name  string
	Email string
}

// FetchMetadata fetches message metadata (UID, ENVELOPE, RFC822.SIZE, FLAGS,
// BODYSTRUCTURE) for the given UID set without altering \Seen flags.
func (cl *Client) FetchMetadata(ctx context.Context, uidSet imap.UIDSet) ([]*MessageMeta, error) {
	fetchCmd := cl.c.Fetch(uidSet, &imap.FetchOptions{
		UID:        true,
		Flags:      true,
		Envelope:   true,
		RFC822Size: true,
		BodyStructure: &imap.FetchItemBodyStructure{
			Extended: true,
		},
	})

	var out []*MessageMeta
	for msg := fetchCmd.Next(); msg != nil; msg = fetchCmd.Next() {
		buf, err := msg.Collect()
		if err != nil {
			continue
		}
		m := metaFromBuffer(buf)
		out = append(out, m)
	}
	if err := fetchCmd.Close(); err != nil {
		return out, fmt.Errorf("fetch metadata close: %w", err)
	}
	return out, nil
}

// FetchBodies downloads the full raw RFC822 bytes (BODY.PEEK[]) for the given
// UIDs and calls fn for each one. fn is called synchronously in fetch order.
func (cl *Client) FetchBodies(ctx context.Context, uidSet imap.UIDSet, fn func(uid imap.UID, raw []byte) error) error {
	fetchCmd := cl.c.Fetch(uidSet, &imap.FetchOptions{
		UID: true,
		BodySection: []*imap.FetchItemBodySection{
			{Peek: true},
		},
	})

	for msg := fetchCmd.Next(); msg != nil; msg = fetchCmd.Next() {
		buf, err := msg.Collect()
		if err != nil {
			continue
		}
		var raw []byte
		if len(buf.BodySection) > 0 {
			raw = buf.BodySection[0].Bytes
		}
		if len(raw) == 0 {
			continue
		}
		if err := fn(buf.UID, raw); err != nil {
			fetchCmd.Close()
			return err
		}
	}
	return fetchCmd.Close()
}

// SearchAll returns all UIDs in the currently selected mailbox.
func (cl *Client) SearchAll(ctx context.Context) ([]imap.UID, error) {
	data, err := cl.c.UIDSearch(&imap.SearchCriteria{}, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("uid search: %w", err)
	}
	return data.AllUIDs(), nil
}

// MarkDeleted flags the given UIDs with \Deleted.
func (cl *Client) MarkDeleted(ctx context.Context, uidSet imap.UIDSet) error {
	storeCmd := cl.c.Store(uidSet, &imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Silent: true,
		Flags:  []imap.Flag{imap.FlagDeleted},
	}, nil)
	for msg := storeCmd.Next(); msg != nil; msg = storeCmd.Next() {
		msg.Collect()
	}
	return storeCmd.Close()
}

// Expunge removes all messages flagged \Deleted in the selected mailbox.
func (cl *Client) Expunge(ctx context.Context) error {
	_, err := cl.c.Expunge().Collect()
	return err
}

// Move performs UID MOVE (with COPY+STORE+EXPUNGE fallback) for the given UIDs.
func (cl *Client) Move(ctx context.Context, uidSet imap.UIDSet, destMailbox string) error {
	if _, err := cl.c.Move(uidSet, destMailbox).Wait(); err != nil {
		// Fallback: COPY then mark \Deleted then EXPUNGE.
		if _, err2 := cl.c.Copy(uidSet, destMailbox).Wait(); err2 != nil {
			return fmt.Errorf("move (copy+delete fallback, copy failed): %w", err2)
		}
		if err2 := cl.MarkDeleted(ctx, uidSet); err2 != nil {
			return fmt.Errorf("move (copy+delete fallback, delete failed): %w", err2)
		}
		return cl.Expunge(ctx)
	}
	return nil
}

// AppendMessage uploads a raw RFC822 message to a mailbox, preserving original
// flags and internal date. Used by the restore operation.
func (cl *Client) AppendMessage(ctx context.Context, mailbox string, raw []byte, flags []imap.Flag, date time.Time) error {
	opts := &imap.AppendOptions{
		Flags: flags,
	}
	if !date.IsZero() {
		opts.Time = date
	}
	cmd := cl.c.Append(mailbox, int64(len(raw)), opts)
	if _, err := io.Copy(cmd, bytes.NewReader(raw)); err != nil {
		return fmt.Errorf("append write: %w", err)
	}
	if err := cmd.Close(); err != nil {
		return fmt.Errorf("append close: %w", err)
	}
	if _, err := cmd.Wait(); err != nil {
		return fmt.Errorf("append: %w", err)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func metaFromBuffer(buf *imapclient.FetchMessageBuffer) *MessageMeta {
	m := &MessageMeta{
		UID:           buf.UID,
		SizeBytes:     buf.RFC822Size,
		Flags:         buf.Flags,
		BodyStructure: buf.BodyStructure,
	}
	if e := buf.Envelope; e != nil {
		m.Date = e.Date
		m.Subject = e.Subject
		m.MessageID = e.MessageID
		if len(e.From) > 0 {
			m.SenderName = e.From[0].Name
			m.SenderEmail = e.From[0].Addr()
		}
		for _, a := range e.To {
			m.Recipients = append(m.Recipients, Address{Name: a.Name, Email: a.Addr()})
		}
		for _, a := range e.Cc {
			m.Recipients = append(m.Recipients, Address{Name: a.Name, Email: a.Addr()})
		}
	} else if !buf.InternalDate.IsZero() {
		m.Date = buf.InternalDate
	}
	return m
}
