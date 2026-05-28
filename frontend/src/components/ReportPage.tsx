import React, { useCallback, useEffect, useState } from 'react';

const authBase = process.env.REACT_APP_AUTH_URL || 'http://localhost:8081';

type SessionInfo = {
  authenticated: boolean;
  access_expires_at?: string;
  needs_consent?: boolean;
  identity_provider?: string;
};

type ReportData = Record<string, unknown>;

const ReportPage: React.FC = () => {
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);
  const [authenticated, setAuthenticated] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [report, setReport] = useState<ReportData | null>(null);

  const checkSession = useCallback(async () => {
    try {
      const res = await fetch(`${authBase}/auth/me`, {
        credentials: 'include',
      });
      if (!res.ok) {
        setAuthenticated(false);
        return;
      }
      const data: SessionInfo = await res.json();
      if (data.authenticated && data.needs_consent) {
        window.location.href = '/consent';
        return;
      }
      setAuthenticated(Boolean(data.authenticated));
    } catch {
      setAuthenticated(false);
    } finally {
      setChecking(false);
    }
  }, []);

  useEffect(() => {
    checkSession();
  }, [checkSession]);

  const login = () => {
    window.location.href = `${authBase}/auth/login`;
  };

  const loginYandex = () => {
    window.location.href = `${authBase}/auth/login/yandex`;
  };

  const logout = async () => {
    await fetch(`${authBase}/auth/logout`, {
      method: 'POST',
      credentials: 'include',
    });
    setAuthenticated(false);
  };

  const fetchReport = async () => {
    try {
      setLoading(true);
      setError(null);
      setReport(null);

      const response = await fetch(`${authBase}/auth/reports`, {
        credentials: 'include',
      });

      if (!response.ok) {
        if (response.status === 403) {
          throw new Error('Доступ к чужому отчёту запрещён');
        }
        if (response.status === 404) {
          throw new Error('Отчёт по вашему пользователю пока не найден');
        }
        const text = await response.text();
        throw new Error(text || `Ошибка запроса (${response.status})`);
      }

      const data = (await response.json()) as ReportData;
      setReport(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
    } finally {
      setLoading(false);
      await checkSession();
    }
  };

  if (checking) {
    return <div className="flex items-center justify-center min-h-screen">Loading...</div>;
  }

  if (!authenticated) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen bg-gray-100 gap-3">
        <button
          onClick={login}
          className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
        >
          Login (Keycloak)
        </button>
        <button
          onClick={loginYandex}
          className="px-4 py-2 bg-red-500 text-white rounded hover:bg-red-600"
        >
          Войти через Яндекс ID
        </button>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center justify-center min-h-screen bg-gray-100">
      <div className="p-8 bg-white rounded-lg shadow-md">
        <h1 className="text-2xl font-bold mb-6">Usage Reports</h1>

        <div className="flex gap-3">
          <button
            onClick={fetchReport}
            disabled={loading}
            className={`px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 ${
              loading ? 'opacity-50 cursor-not-allowed' : ''
            }`}
          >
            {loading ? 'Загрузка отчёта...' : 'Получить отчёт'}
          </button>
          <button
            onClick={logout}
            className="px-4 py-2 bg-gray-500 text-white rounded hover:bg-gray-600"
          >
            Logout
          </button>
        </div>

        {error && (
          <div className="mt-4 p-4 bg-red-100 text-red-700 rounded">{error}</div>
        )}

        {report && (
          <div className="mt-4">
            <h2 className="text-lg font-semibold mb-2">Ваш отчёт</h2>
            <pre className="p-4 bg-gray-100 rounded text-sm overflow-x-auto">
              {JSON.stringify(report, null, 2)}
            </pre>
          </div>
        )}
      </div>
    </div>
  );
};

export default ReportPage;
