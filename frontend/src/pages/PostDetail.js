import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Container,
  Typography,
  Box,
  Button,
  CircularProgress,
  Alert,
  Paper,
  Divider,
} from '@mui/material';
import axios from 'axios';

function PostDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [post, setPost] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [aiAnalysis, setAiAnalysis] = useState(null);
  const [analyzing, setAnalyzing] = useState(false);

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

  const handleDelete = async () => {
    if (window.confirm('Are you sure you want to delete this post?')) {
      try {
        await axios.delete(`http://localhost:8080/posts/${id}`);
        navigate('/');
      } catch (error) {
        setError('Error deleting post. Please try again.');
        console.error('Error deleting post:', error);
      }
    }
  };

  const analyzeContent = async () => {
    setAnalyzing(true);
    try {
      // Here you would integrate with an AI service for content analysis
      // For now, we'll simulate it with a timeout
      await new Promise(resolve => setTimeout(resolve, 2000));
      setAiAnalysis({
        sentiment: 'positive',
        keyPoints: ['Point 1', 'Point 2', 'Point 3'],
        summary: 'This is a sample AI analysis of the post content.',
      });
    } catch (error) {
      setError('Error analyzing content. Please try again.');
      console.error('Error analyzing content:', error);
    } finally {
      setAnalyzing(false);
    }
  };

  if (loading) {
    return (
      <Container sx={{ mt: 4, textAlign: 'center' }}>
        <CircularProgress />
      </Container>
    );
  }

  if (error) {
    return (
      <Container sx={{ mt: 4 }}>
        <Alert severity="error">{error}</Alert>
      </Container>
    );
  }

  if (!post) {
    return (
      <Container sx={{ mt: 4 }}>
        <Alert severity="info">Post not found</Alert>
      </Container>
    );
  }

  return (
    <Container maxWidth="md" sx={{ mt: 4 }}>
      <Paper elevation={3} sx={{ p: 3 }}>
        <Typography variant="h4" component="h1" gutterBottom>
          Post #{post.id}
        </Typography>
        <Typography variant="body1" paragraph>
          {post.body}
        </Typography>
        <Divider sx={{ my: 2 }} />
        <Box sx={{ mt: 2, display: 'flex', gap: 2 }}>
          <Button
            variant="contained"
            color="primary"
            onClick={() => navigate(`/edit/${id}`)}
          >
            Edit
          </Button>
          <Button
            variant="outlined"
            color="error"
            onClick={handleDelete}
          >
            Delete
          </Button>
          <Button
            variant="outlined"
            color="secondary"
            onClick={analyzeContent}
            disabled={analyzing}
          >
            {analyzing ? (
              <>
                <CircularProgress size={24} sx={{ mr: 1 }} />
                Analyzing...
              </>
            ) : (
              'Analyze with AI'
            )}
          </Button>
        </Box>
        {aiAnalysis && (
          <Box sx={{ mt: 4 }}>
            <Typography variant="h6" gutterBottom>
              AI Analysis
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Sentiment: {aiAnalysis.sentiment}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Key Points:
            </Typography>
            <ul>
              {aiAnalysis.keyPoints.map((point, index) => (
                <li key={index}>{point}</li>
              ))}
            </ul>
            <Typography variant="body2" color="text.secondary">
              Summary: {aiAnalysis.summary}
            </Typography>
          </Box>
        )}
      </Paper>
    </Container>
  );
}

export default PostDetail; 