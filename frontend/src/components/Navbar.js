import React from 'react';
import { Link as RouterLink } from 'react-router-dom';
import {
  AppBar,
  Toolbar,
  Typography,
  Button,
  Container,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';

function Navbar() {
  return (
    <AppBar position="static">
      <Container maxWidth="lg">
        <Toolbar>
          <Typography
            variant="h6"
            component={RouterLink}
            to="/"
            sx={{
              flexGrow: 1,
              textDecoration: 'none',
              color: 'inherit',
            }}
          >
            Blog Platform
          </Typography>
          <Button
            component={RouterLink}
            to="/create"
            color="inherit"
            startIcon={<AddIcon />}
          >
            New Post
          </Button>
        </Toolbar>
      </Container>
    </AppBar>
  );
}

export default Navbar; 