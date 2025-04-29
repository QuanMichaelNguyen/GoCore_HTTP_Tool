import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Container,
  TextField,
  Button,
  Box,
  Typography,
  CircularProgress,
  Alert,
} from '@mui/material';
import axios from 'axios';

function EditPost() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [post, setPost] = useState({ body: '' });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [aiSuggesting, setAiSuggesting] = useState(false);

  useEffect(() => {
    fetchPost();
  }, [id]);

  const fetchPost = async () => {
    try {
      const response = await axios.get(`http://localhost:8080/posts/${id}`);
      setPost(response.data);
    } catch (error) {
      setError('Error fetching post. Please try again.');
      console.error('Error fetching post:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSaving(true);
    setError('');

    try {
      await axios.put(`http://localhost:8080/posts/${id}`, post);
      navigate(`/posts/${id}`);
    } catch (error) {
      setError('Error updating post. Please try again.');
      console.error('Error updating post:', error);
    } finally {
      setSaving(false);
    }
  };

  const getAISuggestions = async () => {
    setAiSuggesting(true);
    try {
      // Here you would integrate with an AI service for content suggestions
      // For now, we'll simulate it with a timeout
      await new Promise(resolve => setTimeout(resolve, 2000));
      const suggestions = [
        "Consider adding more details about...",
        "You might want to clarify...",
        "A good example would be...",
      ];
      // In a real implementation, you would show these suggestions in a dialog
      // and let the user choose which ones to apply
      alert(suggestions.join('\n'));
    } catch (error) {
      setError('Error getting AI suggestions. Please try again.');
      console.error('Error getting AI suggestions:', error);
    } finally {
      setAiSuggesting(false);
    }
  };

  if (loading) {
    return (
      <Container sx={{ mt: 4, textAlign: 'center' }}>
        <CircularProgress />
      </Container>
    );
  }

  return (
    <Container maxWidth="md" sx={{ mt: 4 }}>
      <Typography variant="h4" component="h1" gutterBottom>
        Edit Post
      </Typography>
      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}
      <Box component="form" onSubmit={handleSubmit}>
        <TextField
          fullWidth
          multiline
          rows={6}
          label="Post Content"
          value={post.body}
          onChange={(e) => setPost({ ...post, body: e.target.value })}
          margin="normal"
          required
        />
        <Box sx={{ mt: 2, display: 'flex', gap: 2 }}>
          <Button
            type="submit"
            variant="contained"
            color="primary"
            disabled={saving}
          >
            {saving ? <CircularProgress size={24} /> : 'Save Changes'}
          </Button>
          <Button
            variant="outlined"
            color="secondary"
            onClick={getAISuggestions}
            disabled={aiSuggesting}
          >
            {aiSuggesting ? (
              <>
                <CircularProgress size={24} sx={{ mr: 1 }} />
                Getting Suggestions...
              </>
            ) : (
              'Get AI Suggestions'
            )}
          </Button>
        </Box>
      </Box>
    </Container>
  );
}

export default EditPost; 