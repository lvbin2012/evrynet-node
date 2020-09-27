// Copyright 2017 The evrynet-node Authors
// This file is part of the evrynet-node library.
//
// The evrynet-node library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The evrynet-node library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the evrynet-node library. If not, see <http://www.gnu.org/licenses/>.

// fa-only-woff-loader removes the .eot, .ttf, .svg dependencies of the FontAwesome library,
// because they produce unused extra blobs.
module.exports = content => content
	.replace(/src.*url(?!.*url.*(\.eot)).*(\.eot)[^;]*;/, '')
	.replace(/url(?!.*url.*(\.eot)).*(\.eot)[^,]*,/, '')
	.replace(/url(?!.*url.*(\.ttf)).*(\.ttf)[^,]*,/, '')
	.replace(/,[^,]*url(?!.*url.*(\.svg)).*(\.svg)[^;]*;/, ';');
