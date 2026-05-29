import React, { useEffect, useState } from 'react';

const authBase = process.env.REACT_APP_AUTH_URL || 'http://localhost:8081';

const ConsentPage: React.FC = () => {
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const check = async () => {
      try {
        const res = await fetch(`${authBase}/auth/consent/status`, { credentials: 'include' });
        if (!res.ok) {
          window.location.href = '/';
          return;
        }
        const data = await res.json();
        if (!data.needs_consent) {
          window.location.href = '/';
        }
      } catch {
        window.location.href = '/';
      } finally {
        setLoading(false);
      }
    };
    check();
  }, []);

  const accept = async () => {
    try {
      setSubmitting(true);
      setError(null);
      const res = await fetch(`${authBase}/auth/consent`, {
        method: 'POST',
        credentials: 'include',
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Consent failed');
      }
      window.location.href = '/';
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка сохранения');
    } finally {
      setSubmitting(false);
    }
  };

  const decline = async () => {
    await fetch(`${authBase}/auth/logout`, { method: 'POST', credentials: 'include' });
    window.location.href = '/';
  };

  if (loading) {
    return <div className="flex items-center justify-center min-h-screen">Загрузка...</div>;
  }

  return (
    <div className="flex items-center justify-center min-h-screen bg-gray-100">
      <div className="p-8 bg-white rounded-lg shadow-md max-w-lg">
        <h1 className="text-2xl font-bold mb-4">Согласие на использование данных</h1>
        <p className="text-gray-700 mb-4">
          Сервис протезов BionicPRO запрашивает доступ к данным вашего профиля Яндекс ID:
        </p>
        <ul className="list-disc list-inside text-gray-600 mb-6 space-y-1">
          <li>имя и фамилия</li>
          <li>адрес электронной почты</li>
          <li>логин и идентификатор Яндекса</li>
          <li>аватар</li>
        </ul>
        <p className="text-sm text-gray-500 mb-6">
          Данные будут получены через OAuth 2.0 (Identity Brokering Keycloak → Яндекс ID) и сохранены
          в защищённой базе данных сервиса для персонализации работы с протезом.
        </p>
        <div className="flex gap-3">
          <button
            onClick={accept}
            disabled={submitting}
            className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50"
          >
            {submitting ? 'Сохранение...' : 'Разрешить'}
          </button>
          <button
            onClick={decline}
            disabled={submitting}
            className="px-4 py-2 bg-gray-400 text-white rounded hover:bg-gray-500"
          >
            Отказаться
          </button>
        </div>
        {error && <div className="mt-4 p-3 bg-red-100 text-red-700 rounded">{error}</div>}
      </div>
    </div>
  );
};

export default ConsentPage;
