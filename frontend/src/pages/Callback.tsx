import { useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import api from '../services/api';

const Callback = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { login } = useAuth();

  useEffect(() => {
    const code = searchParams.get('code');
    if (code) {
      api.post('/auth/discord', { code })
        .then((response) => {
          login(response.data.token);
          navigate('/');
        })
        .catch((error) => {
          console.error('Login failed', error);
          navigate('/login');
        });
    } else {
      navigate('/login');
    }
  }, [searchParams, login, navigate]);

  return (
    <div className="flex justify-center items-center h-screen">
      <p className="text-xl">Logging in...</p>
    </div>
  );
};

export default Callback;
