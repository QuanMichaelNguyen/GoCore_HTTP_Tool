import React, { useState, useEffect } from 'react';
import { Link as RouterLink } from 'react-router-dom';
import { useLocation } from 'react-router-dom';
import {
  Container,
  Grid,
  Card,
  CardContent,
  Typography,
  Button,
  Pagination,
  Box,
  Alert,
  CircularProgress,
} from '@mui/material';
import axios from 'axios';

function Home() {
  const [posts, setPosts] = useState([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const limit = 10;
  const location = useLocation();

  // Combined useEffect to handle all refresh conditions
  useEffect(() => {
    // Log the location state for debugging
    console.log('Location state:', location.state);
    
    // Check any condition that should trigger a refresh
    if (location.state?.newPostCreated || location.state?.refresh) {
      console.log('Refreshing posts due to state change');
      fetchPosts();
      // Clear the navigation state to prevent infinite refreshes
      window.history.replaceState({}, document.title);
    }
  }, [location.state]);

  // This useEffect will run when the page changes
  useEffect(() => {
    fetchPosts();
  }, [page]);

  const fetchPosts = async () => {
    try {
      setLoading(true);
      console.log(`Fetching posts with limit=${limit} and offset=${(page - 1) * limit}`);
      
      const response = await axios.get(`http://localhost:8080/posts?limit=${limit}&offset=${(page - 1) * limit}`);
      console.log('API Response:', response.data);
      
      // Check if response.data is directly an array of posts
      if (Array.isArray(response.data)) {
        setPosts(response.data);
        // Since we don't have total count, we'll use the current page if posts are full
        const isLastPage = response.data.length < limit;
        setTotalPages(isLastPage ? page : page + 1);
      } 
      // Otherwise check if it's an object with posts property
      else if (Array.isArray(response.data.posts)) {
        setPosts(response.data.posts);
        setTotalPages(Math.ceil((response.data.totalPosts || 0) / limit));
      } 
      // Neither format matches - log error
      else {
        console.error('Could not parse posts from response:', response.data);
        setPosts([]);
        setTotalPages(1);
      }
      
      setError('');
    } catch (error) {
      console.error('Error fetching posts:', error);
      setError('Failed to fetch posts. Please try again later.');
      setPosts([]);
    } finally {
      setLoading(false);
    }
  };

  const handlePageChange = (event, value) => {
    setPage(value);
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

  return (
    <Container maxWidth="lg" sx={{ mt: 4 }}>
      <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Typography variant="h4" component="h1">Posts</Typography>
        <Button 
          component={RouterLink} 
          to="/create-post" 
          variant="contained" 
          color="primary"
        >
          Create New Post
        </Button>
      </Box>
      
      {posts.length === 0 ? (
        <Alert severity="info">No posts found. Create your first post!</Alert>
      ) : (
        <>
          <Grid container spacing={3}>
            {posts.map((post) => (
              <Grid item xs={12} key={post.id}>
                <Card>
                  <CardContent>
                    <Typography variant="h5" component="div">
                      {post.body ? post.body.substring(0, 100) + (post.body.length > 100 ? '...' : '') : 'No content'}
                    </Typography>
                    <Box sx={{ mt: 2 }}>
                      <Button
                        component={RouterLink}
                        to={`/posts/${post.id}`}
                        variant="contained"
                        color="primary"
                      >
                        Read More
                      </Button>
                    </Box>
                  </CardContent>
                </Card>
              </Grid>
            ))}
          </Grid>
          <Box sx={{ mt: 4, display: 'flex', justifyContent: 'center' }}>
            <Pagination
              count={totalPages}
              page={page}
              onChange={handlePageChange}
              color="primary"
            />
          </Box>
        </>
      )}
    </Container>
  );
}

export default Home;