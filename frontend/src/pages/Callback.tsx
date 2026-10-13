import { useEffect, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { MetaTags } from '../hooks/useMetaTags';
import api from '../services/api';

const Callback = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { login } = useAuth();
  // Discord codes are single-use. login() changes auth state, which re-renders
  // this component and re-fires the effect, so the code would be spent twice.
  const exchanged = useRef(false);

  useEffect(() => {
    if (exchanged.current) return;
    const code = searchParams.get('code');
    if (code) {
      exchanged.current = true;
      api.post('/auth/discord', { code })
        .then((response) => {
          login(response.data.token);
          
          // Redirect to original location or home
          const redirectPath = localStorage.getItem('auth_redirect') || '/';
          localStorage.removeItem('auth_redirect');
          navigate(redirectPath);
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
      <MetaTags title="Signing in" description="Completing Discord sign-in." noindex />
      <p className="text-xl">Logging in...</p>
    </div>
  );
};

export default Callback;
