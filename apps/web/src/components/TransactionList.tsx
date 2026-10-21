import React from 'react';
import { Copy, Eye, Search } from 'lucide-react';
import { formatINR } from '../lib/api';
import type { Payment } from '../lib/api';

interface TransactionListProps {
  payments: Payment[];
  selectedStatus: string;
  onSelectStatus: (status: string) => void;
  onInspect: (payment: Payment) => void;
  autoRefresh: boolean;
  onToggleAutoRefresh: () => void;
  searchQuery: string;
  onSearchChange: (q: string) => void;
}

const STATUS_FILTERS = ['ALL', 'CREATED', 'PENDING', 'SUCCEEDED', 'FAILED', 'UNKNOWN'];

export const TransactionList: React.FC<TransactionListProps> = ({
  payments,
  selectedStatus,
  onSelectStatus,
  onInspect,
  autoRefresh,
  onToggleAutoRefresh,
  searchQuery,
  onSearchChange,
}) => {
  const filtered = payments.filter((p) => {
    const matchesStatus = selectedStatus === 'ALL' || p.status === selectedStatus;
    const query = searchQuery.toLowerCase().trim();
    const matchesQuery =
      !query ||
      p.id.toLowerCase().includes(query) ||
      p.idempotency_key.toLowerCase().includes(query) ||
      (p.description && p.description.toLowerCase().includes(query));
    return matchesStatus && matchesQuery;
  });

  const copyToClipboard = (text: string, e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(text);
  };

  return (
    <div className="glass-panel" style={{ padding: '24px' }}>
      {/* List Header & Controls */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 16,
          marginBottom: 20,
        }}
      >
        <div>
          <h2 style={{ fontSize: 18, fontWeight: 600, color: '#F8FAFC', marginBottom: 4 }}>
            Transaction Ledger
          </h2>
          <p style={{ fontSize: 13, color: 'var(--text-dim)' }}>
            Real-time feed of payments processed through Go engine
          </p>
        </div>

        {/* Search & Auto-Refresh Toggle */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          {/* Search Input */}
          <div style={{ position: 'relative', width: 220 }}>
            <Search
              size={15}
              style={{
                position: 'absolute',
                left: 10,
                top: '50%',
                transform: 'translateY(-50%)',
                color: 'var(--text-dim)',
              }}
            />
            <input
              type="text"
              placeholder="Search ID / Key..."
              className="form-input"
              value={searchQuery}
              onChange={(e) => onSearchChange(e.target.value)}
              style={{ paddingLeft: 32, fontSize: 13, padding: '7px 10px 7px 32px' }}
            />
          </div>

          {/* Auto Refresh pill */}
          <label
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              fontSize: 12,
              color: autoRefresh ? 'var(--emerald-400)' : 'var(--text-dim)',
              background: 'rgba(15, 23, 42, 0.6)',
              padding: '6px 12px',
              borderRadius: 9999,
              border: '1px solid var(--border-subtle)',
              cursor: 'pointer',
              userSelect: 'none',
            }}
          >
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={onToggleAutoRefresh}
              style={{ accentColor: 'var(--emerald-500)', cursor: 'pointer' }}
            />
            <span>Auto-poll (3s)</span>
          </label>
        </div>
      </div>

      {/* Filter Tabs */}
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 20 }}>
        {STATUS_FILTERS.map((status) => (
          <button
            key={status}
            onClick={() => onSelectStatus(status)}
            style={{
              background: selectedStatus === status ? 'rgba(16, 185, 129, 0.15)' : 'rgba(30, 41, 59, 0.4)',
              color: selectedStatus === status ? 'var(--emerald-400)' : 'var(--text-muted)',
              border: `1px solid ${selectedStatus === status ? 'rgba(16, 185, 129, 0.4)' : 'transparent'}`,
              padding: '5px 12px',
              borderRadius: 9999,
              fontSize: 12,
              fontWeight: 500,
              cursor: 'pointer',
              transition: 'all 0.15s ease',
            }}
          >
            {status}
          </button>
        ))}
      </div>

      {/* Table / List */}
      {filtered.length === 0 ? (
        <div
          style={{
            padding: '48px 24px',
            textAlign: 'center',
            color: 'var(--text-dim)',
          }}
        >
          <div style={{ fontSize: 15, fontWeight: 500, color: 'var(--text-muted)', marginBottom: 6 }}>
            No transactions found
          </div>
          <p style={{ fontSize: 13 }}>
            Create a payment or run the Idempotency Lab to generate live records.
          </p>
        </div>
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 13 }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-subtle)', color: 'var(--text-dim)' }}>
                <th style={{ padding: '10px 12px', fontWeight: 500 }}>Payment ID</th>
                <th style={{ padding: '10px 12px', fontWeight: 500 }}>Amount</th>
                <th style={{ padding: '10px 12px', fontWeight: 500 }}>Status</th>
                <th style={{ padding: '10px 12px', fontWeight: 500 }}>Idempotency Key</th>
                <th style={{ padding: '10px 12px', fontWeight: 500 }}>Created</th>
                <th style={{ padding: '10px 12px', fontWeight: 500, textAlign: 'right' }}>Action</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((p) => (
                <tr
                  key={p.id}
                  onClick={() => onInspect(p)}
                  style={{
                    borderBottom: '1px solid rgba(51, 65, 85, 0.25)',
                    cursor: 'pointer',
                    transition: 'background 0.12s ease',
                  }}
                  onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'rgba(30, 41, 59, 0.35)')}
                  onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                >
                  {/* ID */}
                  <td style={{ padding: '12px 12px', fontFamily: 'var(--font-mono)', color: '#F8FAFC' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      <span>{p.id}</span>
                      <button
                        onClick={(e) => copyToClipboard(p.id, e)}
                        title="Copy ID"
                        style={{
                          background: 'none',
                          border: 'none',
                          color: 'var(--text-dim)',
                          cursor: 'pointer',
                          display: 'flex',
                        }}
                      >
                        <Copy size={12} />
                      </button>
                    </div>
                  </td>

                  {/* Amount */}
                  <td style={{ padding: '12px 12px', fontWeight: 600, color: 'var(--emerald-400)' }}>
                    {formatINR(p.amount)}
                  </td>

                  {/* Status */}
                  <td style={{ padding: '12px 12px' }}>
                    <span className={`status-badge ${p.status}`}>{p.status}</span>
                  </td>

                  {/* Idempotency Key */}
                  <td style={{ padding: '12px 12px', fontFamily: 'var(--font-mono)', color: 'var(--text-muted)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      <span style={{ maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {p.idempotency_key}
                      </span>
                      <button
                        onClick={(e) => copyToClipboard(p.idempotency_key, e)}
                        title="Copy key"
                        style={{
                          background: 'none',
                          border: 'none',
                          color: 'var(--text-dim)',
                          cursor: 'pointer',
                          display: 'flex',
                        }}
                      >
                        <Copy size={12} />
                      </button>
                    </div>
                  </td>

                  {/* Created At */}
                  <td style={{ padding: '12px 12px', color: 'var(--text-dim)' }}>
                    {new Date(p.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                  </td>

                  {/* Action */}
                  <td style={{ padding: '12px 12px', textAlign: 'right' }}>
                    <button
                      className="btn btn-secondary"
                      onClick={(e) => {
                        e.stopPropagation();
                        onInspect(p);
                      }}
                      style={{ padding: '5px 10px', fontSize: 12 }}
                    >
                      <Eye size={13} />
                      Inspect
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
